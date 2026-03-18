"""Project discovery and classification tools."""

import json
import os


SCAN_ROOTS = ["E:/"]

SKIP_DIRS = {
    "$RECYCLE.BIN", "System Volume Information", "OneDriveTemp",
    "WizTree", "Software_软件", "Huawei Share", "Articles",
    "Creds", "DS", "ML_edu", "Politech", "Portfolio",
    "Diser", "Sirius",
}

PROJECT_MARKERS = [
    ".git", "docker-compose.yml", "docker-compose.yaml",
    "Dockerfile", "package.json", "go.mod",
    "requirements.txt", "pom.xml", "build.gradle",
]


def scan_projects(root: str = "E:/", check_registry: bool = True) -> dict:
    """Scan a directory for unregistered projects.

    Identifies directories that look like development projects
    (have .git, Dockerfile, package.json etc.) but are not in
    agent-context registry.json.

    Args:
        root:           Root directory to scan (default E:/).
        check_registry: If True, filter out already-registered projects.

    Returns dict with found projects, already_registered, and scan summary.

    NOTE: This tool runs on the server and cannot directly read E:\\.
    It works by reading the registry and cross-referencing with the list
    of known project paths. Use with Filesystem MCP scan results.
    """
    registry_path = "/opt/agent-context/data/registry.json"
    registered_paths = set()
    try:
        with open(registry_path) as f:
            reg = json.load(f)
            registered_paths = {
                p.replace("\\", "/").lower().rstrip("/")
                for p in reg.get("projects", {}).keys()
            }
    except Exception:
        pass

    return {
        "registered_paths": list(registered_paths),
        "registry_path":    registry_path,
        "scan_note": (
            "Use Filesystem MCP to list E:\\ directories, "
            "then call identify_project() for each, "
            "then cross-reference with registered_paths above."
        )
    }


def identify_project(path: str, git_remote: str | None = None) -> dict:
    """Analyze a directory and classify it as a project.

    Called after Filesystem MCP lists E:\\ contents. Classifies
    a directory based on known markers and git remote URL.

    Args:
        path:       Full path on Windows (e.g. 'E:/Sharmanka').
        git_remote: Git remote URL if known (from .git/config).

    Returns project classification: type, name, suggested_config.
    """
    name = path.rstrip("/\\").split("/")[-1].split("\\")[-1]
    project_type = _classify_project(name, git_remote or "")
    suggested_compose_project = name.lower().replace("-", "_").replace(" ", "_")

    return {
        "path":                 path,
        "name":                 name,
        "type":                 project_type,
        "git_remote":           git_remote,
        "suggested_registry_entry": {
            "name":             name,
            "type":             project_type,
            "description":      f"{name} project",
            "compose_projects": [suggested_compose_project],
        },
        "needs_mcp_json":    True,
        "needs_claude_md":   True,
        "needs_server_deploy": None,
    }


def _classify_project(name: str, remote: str) -> str:
    """Guess project type from name and git remote."""
    name_lower = name.lower()

    if any(kw in name_lower for kw in ("bot", "telegram")):
        return "bot"
    if any(kw in name_lower for kw in ("api", "backend", "server")):
        return "backend"
    if any(kw in name_lower for kw in ("frontend", "ui", "dashboard")):
        return "frontend"
    if any(kw in name_lower for kw in ("mcp", "agent", "context")):
        return "infra"
    if any(kw in name_lower for kw in ("ml", "ai", "rag", "model")):
        return "experiment"
    return "experiment"
