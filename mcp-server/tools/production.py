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
