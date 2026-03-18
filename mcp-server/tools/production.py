"""Production monitoring — aggregate status across all production projects."""

import time

from .services import get_services_status, _resolve_container
from .cache import get_recent_events
from .git_tools import git_status, GIT_REPOS
from .workflows import PROJECT_CONFIG


PRODUCTION_PROJECTS = ["locoll", "errorlens", "moex", "rag-qa", "scout", "warehouse"]


def _group_by_project(services: list[dict]) -> dict[str, list[dict]]:
    """Group services list by project prefix."""
    groups: dict[str, list[dict]] = {}
    for svc in services:
        name = svc["name"]
        # Определяем проект по префиксу имени сервиса
        project = None
        for proj in PRODUCTION_PROJECTS:
            prefix = proj.replace("-", "")  # rag-qa -> ragqa
            if name.startswith(proj) or name.startswith(prefix):
                project = proj
                break
        if project:
            groups.setdefault(project, []).append(svc)
    return groups


def get_production_status(since_hours: int = 24) -> dict:
    """Aggregate health status across all production projects.

    Returns per-project summary: services up/down, recent events,
    git deploy status, and overall health verdict.

    Use this for a single-call production overview.

    Args:
        since_hours: How far back to look for events (default 24).
    """
    start = time.monotonic()

    # 1. Все сервисы
    all_services = get_services_status()
    grouped = _group_by_project(all_services)

    # 2. Per-project summary
    projects = {}
    total_up = 0
    total_down = 0

    for proj in PRODUCTION_PROJECTS:
        svcs = grouped.get(proj, [])
        up = [s for s in svcs if s.get("is_up")]
        down = [s for s in svcs if not s.get("is_up")]
        total_up += len(up)
        total_down += len(down)

        # Events
        events = get_recent_events(hours=since_hours, project=proj)

        # Git status
        git = git_status(proj) if proj in GIT_REPOS else None

        projects[proj] = {
            "services_up": len(up),
            "services_down": len(down),
            "down_names": [s["name"] for s in down],
            "events_count": len(events),
            "events_last3": events[:3],
            "git_branch": git.get("branch") if git else None,
            "git_commit": git.get("last_commit", {}).get("short_hash") if git else None,
            "git_message": git.get("last_commit", {}).get("message") if git else None,
            "git_clean": git.get("is_clean") if git else None,
        }

    elapsed = round(time.monotonic() - start, 1)

    # Overall verdict
    if total_down == 0:
        verdict = "ALL_GREEN"
    elif total_down <= 2:
        verdict = "DEGRADED"
    else:
        verdict = "CRITICAL"

    return {
        "verdict": verdict,
        "total_services_up": total_up,
        "total_services_down": total_down,
        "projects": projects,
        "since_hours": since_hours,
        "check_time_s": elapsed,
    }


def _verify_deploy(project: str) -> dict:
    """Verify deploy: health check + recent logs."""
    import subprocess
    import httpx

    cfg = PROJECT_CONFIG.get(project, {})
    result = {"project": project}

    # Health check
    health_url = cfg.get("health_url")
    if health_url:
        try:
            resp = httpx.get(health_url, timeout=10)
            result["health"] = {
                "status_code": resp.status_code,
                "healthy": resp.status_code < 400,
                "body": resp.text[:200],
            }
        except Exception as e:
            result["health"] = {"healthy": False, "error": str(e)}
    else:
        result["health"] = {"healthy": None, "note": "No health endpoint configured"}

    # Recent logs
    main_service = cfg.get("main_service", "")
    if main_service:
        container = f"{project}-{main_service}-1"
        logs_result = subprocess.run(
            ["docker", "logs", "--tail", "20", container],
            capture_output=True, text=True, timeout=15
        )
        log_text = (logs_result.stdout + logs_result.stderr).strip()[-3000:]
        has_errors = any(
            kw in log_text.lower()
            for kw in ["error", "traceback", "exception", "fatal"]
        )
        result["logs"] = {
            "container": container,
            "has_errors": has_errors,
            "tail": log_text,
        }
    else:
        result["logs"] = {"note": "No main_service configured"}

    return result


def notify_deploy(
    project: str,
    commit: str = "",
    message: str = "",
    branch: str = "main",
    status: str = "success",
) -> dict:
    """Notify LocOll about a deploy and run automatic verification.

    Called by project agents after deploy_project() completes.
    Records event, runs health check + log inspection, returns verdict.

    Args:
        project:  Project name (e.g. 'moex', 'errorlens', 'warehouse').
        commit:   Short commit hash (optional, auto-detected if empty).
        message:  Commit message (optional, auto-detected if empty).
        branch:   Branch deployed (default 'main').
        status:   Deploy status from caller: 'success' or 'failed'.
    """
    from .cache import record_event

    start = time.monotonic()

    # Auto-detect commit info if not provided
    if not commit and project in GIT_REPOS:
        git = git_status(project)
        if git and not git.get("error"):
            commit = git.get("last_commit", {}).get("short_hash", "")
            message = message or git.get("last_commit", {}).get("message", "")

    # Record deploy event
    detail = f"branch={branch} commit={commit} status={status}"
    if message:
        detail += f" msg={message}"
    record_event(
        container=f"{project}-deploy",
        event="deploy",
        project=project,
        detail=detail,
    )

    # Verify if deploy was reported as success
    verification = None
    if status == "success":
        verification = _verify_deploy(project)

    elapsed = round(time.monotonic() - start, 1)

    # Verdict
    if status != "success":
        verdict = "DEPLOY_FAILED"
    elif verification:
        health_ok = verification.get("health", {}).get("healthy") is not False
        logs_ok = not verification.get("logs", {}).get("has_errors", False)
        if health_ok and logs_ok:
            verdict = "VERIFIED_OK"
        elif health_ok:
            verdict = "WARNINGS_IN_LOGS"
        else:
            verdict = "HEALTH_FAILED"
    else:
        verdict = "NOT_VERIFIED"

    return {
        "project": project,
        "verdict": verdict,
        "commit": commit,
        "message": message,
        "branch": branch,
        "deploy_status": status,
        "verification": verification,
        "check_time_s": elapsed,
    }
