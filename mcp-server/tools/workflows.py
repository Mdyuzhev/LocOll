"""Workflow tools — composite operations for common deployment and maintenance tasks."""

import subprocess
import time
import shlex


PROJECT_CONFIG = {
    "locoll": {
        "repo_path":    "/opt/locoll",
        "compose_dir":  "/opt/locoll",
        "health_url":   "http://localhost:4000/api/v1/health",
        "main_service": "backend",
    },
    "moex": {
        "repo_path":    "/home/flomaster/moex-bot",
        "compose_dir":  "/home/flomaster/moex-bot",
        "health_url":   None,
        "main_service": "moex-bot",
    },
    "errorlens": {
        "repo_path":    "/opt/errorlens",
        "compose_dir":  "/opt/errorlens/docker",
        "health_url":   "http://localhost:8002/health",
        "main_service": "backend",
    },
    "warehouse": {
        "repo_path":    "/opt/warehouse",
        "compose_dir":  "/opt/warehouse",
        "health_url":   "http://localhost:8080/actuator/health",
        "main_service": "api",
    },
    "homelab-mcp": {
        "repo_path":    "/opt/homelab-mcp",
        "compose_dir":  "/opt/homelab-mcp",
        "compose_file": "docker-compose.mcp.yml",
        "health_url":   "http://localhost:8765/health",
        "main_service": "",
    },
    "rag-qa": {
        "repo_path":    "/home/flomaster/rag-qa",
        "compose_dir":  "/home/flomaster/rag-qa",
        "health_url":   "http://localhost:8001/health",
        "main_service": "",
    },
}


def deploy_project(
    project: str,
    service: str = "",
    build: bool = False,
    log_lines: int = 30,
) -> dict:
    """Deploy a project: git pull → compose up → health check → tail logs.

    One call replaces the typical 3-4 step deployment sequence.
    Returns structured report: each step result, health status, log tail.

    Args:
        project: Project name: 'locoll', 'moex', 'errorlens', 'warehouse',
                 'homelab-mcp', 'rag-qa'.
        service: Specific compose service to restart (empty = all services).
                 Pass service name to avoid rebuilding unrelated containers.
        build:   Rebuild Docker image before starting (default False).
                 Use True only when requirements.txt or Dockerfile changed.
        log_lines: Lines of logs to include in response (default 30).

    Returns dict with steps (git_pull, compose_up, health, logs),
    success flag, and total_time_s.
    """
    if project not in PROJECT_CONFIG:
        return {
            "success": False,
            "error": f"Unknown project '{project}'. Known: {list(PROJECT_CONFIG.keys())}",
        }

    cfg = PROJECT_CONFIG[project]
    steps = {}
    total_start = time.monotonic()

    # Step 1: git pull
    pull_result = subprocess.run(
        f"cd {shlex.quote(cfg['repo_path'])} && git pull 2>&1",
        shell=True, capture_output=True, text=True, timeout=60
    )
    steps["git_pull"] = {
        "exit_code": pull_result.returncode,
        "output":    pull_result.stdout.strip()[-2000:],
    }

    # Step 2: compose up
    compose_file_flag = f"-f {cfg['compose_file']}" if cfg.get("compose_file") else ""
    svc = shlex.quote(service) if service else (cfg.get("main_service", "") or "")
    build_flag = "--build" if build else ""

    compose_cmd = (
        f"cd {shlex.quote(cfg['compose_dir'])} && "
        f"docker compose {compose_file_flag} up -d {build_flag} {svc} 2>&1"
    ).strip()

    compose_result = subprocess.run(
        compose_cmd, shell=True, capture_output=True, text=True,
        timeout=300 if build else 60
    )
    steps["compose_up"] = {
        "exit_code": compose_result.returncode,
        "output":    compose_result.stdout.strip()[-3000:],
    }

    # Step 3: health check
    health_url = cfg.get("health_url")
    if health_url:
        import httpx
        time.sleep(3)
        try:
            resp = httpx.get(health_url, timeout=10)
            steps["health"] = {
                "status_code": resp.status_code,
                "healthy":     resp.status_code < 400,
                "body":        resp.text[:200],
            }
        except Exception as e:
            steps["health"] = {"healthy": False, "error": str(e)}
    else:
        steps["health"] = {"healthy": None, "note": "No health endpoint configured"}

    # Step 4: tail logs
    container_name = (
        f"{project}-{service or cfg.get('main_service', '')}-1"
        if (service or cfg.get("main_service"))
        else None
    )
    if container_name:
        logs_result = subprocess.run(
            ["docker", "logs", "--tail", str(log_lines), container_name],
            capture_output=True, text=True, timeout=15
        )
        steps["logs"] = {
            "container": container_name,
            "content":   (logs_result.stdout + logs_result.stderr).strip()[-5000:],
        }
    else:
        steps["logs"] = {"note": "No main_service configured, skipping logs"}

    total_time = round(time.monotonic() - total_start, 1)
    all_ok = (
        steps["git_pull"]["exit_code"] == 0 and
        steps["compose_up"]["exit_code"] == 0 and
        steps.get("health", {}).get("healthy", True) is not False
    )

    return {
        "project":      project,
        "success":      all_ok,
        "total_time_s": total_time,
        "steps":        steps,
        "locoll_hint": (
            f"Call notify_deploy('{project}') to hand off to LocOll for verification"
            if all_ok else
            f"Deploy had issues. Call notify_deploy('{project}', status='failed') to report."
        ),
    }


def restart_and_verify(service: str, wait_seconds: int = 5, log_lines: int = 20) -> dict:
    """Restart a container and verify it came back healthy.

    Replaces the 3-step pattern: restart → wait → check logs.
    Returns status before/after, health check result, and log tail.

    Args:
        service:      Container name or alias (fuzzy match supported).
        wait_seconds: How long to wait after restart before checking (default 5).
        log_lines:    Lines of logs to include in response (default 20).
    """
    from .services import _resolve_container
    from .docker_tools import _get_container_status

    container = _resolve_container(service)
    start = time.monotonic()

    # Status before
    status_before = _get_container_status(container)

    # Restart
    restart_result = subprocess.run(
        ["docker", "restart", container],
        capture_output=True, text=True, timeout=60
    )
    if restart_result.returncode != 0:
        return {
            "container": container,
            "success":   False,
            "error":     restart_result.stderr.strip(),
        }

    # Wait
    time.sleep(wait_seconds)

    # Status after
    status_after = _get_container_status(container)

    # Health via docker inspect
    health_result = subprocess.run(
        ["docker", "inspect", "--format",
         "{{.State.Status}}:{{.State.Health.Status}}", container],
        capture_output=True, text=True, timeout=10
    )
    raw = health_result.stdout.strip()
    state, _, health = raw.partition(":")
    health_str = health if health and health != "<no value>" else "no healthcheck"

    # Log tail
    logs_result = subprocess.run(
        ["docker", "logs", "--tail", str(log_lines), container],
        capture_output=True, text=True, timeout=15
    )
    log_tail = (logs_result.stdout + logs_result.stderr).strip()[-5000:]

    elapsed = round(time.monotonic() - start, 1)
    is_healthy = state == "running" and health_str != "unhealthy"

    return {
        "container":      container,
        "success":        is_healthy,
        "status_before":  status_before,
        "status_after":   status_after,
        "health":         health_str,
        "total_time_s":   elapsed,
        "log_tail":       log_tail,
    }


MIGRATION_CONFIG = {
    "errorlens": {
        "container":   "errorlens-backend-1",
        "check_cmd":   "cd /app && alembic current 2>&1",
        "migrate_cmd": "cd /app && alembic upgrade head 2>&1",
    },
    "rag-qa": {
        "container":   "rag-qa-api-1",
        "check_cmd":   "cd /app && alembic current 2>&1",
        "migrate_cmd": "cd /app && alembic upgrade head 2>&1",
    },
    "warehouse": {
        "container":   "warehouse-api-1",
        "check_cmd":   "ls /app/migrations/",
        "migrate_cmd": None,
    },
}


def run_db_migration(project: str, dry_run: bool = False) -> dict:
    """Run database migrations inside a project's container.

    Checks current migration state, runs upgrade, then verifies result.
    Supports Alembic (errorlens, rag-qa). Warehouse uses Flyway via CI.

    Args:
        project:  Project name: 'errorlens', 'rag-qa', 'warehouse'.
        dry_run:  If True — only show current state, don't run migration.
    """
    if project not in MIGRATION_CONFIG:
        return {
            "success": False,
            "error": (
                f"Unknown project '{project}'. "
                f"Supported: {list(MIGRATION_CONFIG.keys())}"
            ),
        }

    cfg = MIGRATION_CONFIG[project]
    container = cfg["container"]

    # Step 1: current state
    check_before = subprocess.run(
        ["docker", "exec", container, "sh", "-c", cfg["check_cmd"]],
        capture_output=True, text=True, timeout=30
    )
    state_before = (check_before.stdout + check_before.stderr).strip()

    if dry_run:
        return {
            "project":       project,
            "dry_run":       True,
            "current_state": state_before,
        }

    if not cfg.get("migrate_cmd"):
        return {
            "project": project,
            "success": False,
            "note":    "Migration for this project runs via CI only. Trigger via git push.",
            "current_state": state_before,
        }

    # Step 2: run migration
    migrate_result = subprocess.run(
        ["docker", "exec", container, "sh", "-c", cfg["migrate_cmd"]],
        capture_output=True, text=True, timeout=120
    )
    migration_output = (migrate_result.stdout + migrate_result.stderr).strip()

    # Step 3: state after
    check_after = subprocess.run(
        ["docker", "exec", container, "sh", "-c", cfg["check_cmd"]],
        capture_output=True, text=True, timeout=30
    )
    state_after = (check_after.stdout + check_after.stderr).strip()

    return {
        "project":          project,
        "success":          migrate_result.returncode == 0,
        "state_before":     state_before,
        "migration_output": migration_output[-3000:],
        "state_after":      state_after,
    }
