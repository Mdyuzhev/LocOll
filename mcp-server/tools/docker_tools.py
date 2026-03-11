"""Tools for Docker container management."""

import subprocess
import json
import time


def get_docker_ps() -> list[dict]:
    """Get all Docker containers with their status.

    Returns structured list: id, name, image, status, ports, created.
    """
    fmt = '{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","ports":"{{.Ports}}","created":"{{.CreatedAt}}"}'
    result = subprocess.run(
        ["docker", "ps", "-a", "--format", fmt],
        capture_output=True, text=True, timeout=15
    )
    containers = []
    for line in result.stdout.strip().split("\n"):
        if line.strip():
            try:
                containers.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return containers


def restart_service(service: str) -> dict:
    """Restart a Docker container by name.

    Args:
        service: Container name (e.g. 'warehouse-api', 'locoll-backend-1')

    Returns status before and after restart, and operation time.
    """
    from .services import _resolve_container

    container = _resolve_container(service)

    # Status before
    before = _get_container_status(container)

    start = time.monotonic()
    result = subprocess.run(
        ["docker", "restart", container],
        capture_output=True, text=True, timeout=60
    )
    elapsed = round(time.monotonic() - start, 1)

    if result.returncode != 0:
        return {
            "container": container,
            "success": False,
            "error": result.stderr.strip(),
        }

    # Status after
    after = _get_container_status(container)

    return {
        "container": container,
        "success": True,
        "status_before": before,
        "status_after": after,
        "operation_time_s": elapsed,
    }


def stop_service(service: str) -> dict:
    """Stop a Docker container by name.

    Args:
        service: Container name (e.g. 'warehouse-api', 'locoll-backend-1')
    """
    from .services import _resolve_container

    container = _resolve_container(service)
    result = subprocess.run(
        ["docker", "stop", container],
        capture_output=True, text=True, timeout=30
    )
    return {
        "container": container,
        "success": result.returncode == 0,
        "output": result.stdout.strip() or result.stderr.strip(),
    }


def start_service(service: str) -> dict:
    """Start a Docker container by name.

    Args:
        service: Container name (e.g. 'warehouse-api', 'locoll-backend-1')
    """
    from .services import _resolve_container

    container = _resolve_container(service)
    result = subprocess.run(
        ["docker", "start", container],
        capture_output=True, text=True, timeout=30
    )
    return {
        "container": container,
        "success": result.returncode == 0,
        "output": result.stdout.strip() or result.stderr.strip(),
    }


def _get_container_status(container: str) -> str:
    result = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Status}}", container],
        capture_output=True, text=True, timeout=10
    )
    return result.stdout.strip() if result.returncode == 0 else "unknown"
