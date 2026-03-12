"""Tools for service status, logs, and system metrics."""

import subprocess
import time
import httpx


SERVICES = {
    "locoll-backend": {"port": 8010, "health": "http://localhost:8010/api/v1/health"},
    "locoll-nginx": {"port": 4000, "health": "http://localhost:4000/"},
    "warehouse-api": {"port": 8080, "health": "http://localhost:8080/health"},
    "warehouse-grafana": {"port": 3001, "health": "http://localhost:3001/"},
    "warehouse-prometheus": {"port": 9090, "health": "http://localhost:9090/-/healthy"},
    "warehouse-frontend": {"port": 80, "health": "http://localhost:80/"},
    "warehouse-telegram-bot": {"port": 8000, "health": None},
    "warehouse-robot": {"port": 8070, "health": "http://localhost:8070/health"},
    "warehouse-analytics": {"port": 8090, "health": None},
    "warehouse-uplink-bot": {"port": 8001, "health": None},
    "errorlens-backend": {"port": 8002, "health": "http://localhost:8002/health"},
    "errorlens-nginx": {"port": 3000, "health": "http://localhost:3000/"},
    "errorlens-generator": {"port": None, "health": None, "container": "docker-generator-1"},
    "errorlens-notification": {"port": None, "health": None, "container": "docker-notification-worker-1"},
    "errorlens-automation": {"port": None, "health": None, "container": "docker-automation-worker-1"},
    "errorlens-gitlab": {"port": 8929, "health": None},
    "ollama": {"port": 11434, "health": "http://localhost:11434/"},
}

# Map short names to docker container name patterns
CONTAINER_ALIASES = {
    "locoll": "locoll-backend",
    "locoll-backend": "locoll-backend",
    "locoll-nginx": "locoll-nginx",
    "warehouse-api": "warehouse-api",
    "warehouse-grafana": "warehouse-grafana",
    "warehouse-prometheus": "warehouse-prometheus",
    "warehouse-frontend": "warehouse-frontend",
    "warehouse-telegram-bot": "warehouse-telegram-bot",
    "warehouse-robot": "warehouse-robot",
    "warehouse-analytics": "warehouse-analytics",
    "warehouse-uplink-bot": "warehouse-uplink-bot",
    "warehouse-redis": "warehouse-redis",
    "warehouse-kafka": "warehouse-kafka",
    "warehouse-db": "warehouse-db",
    "errorlens": "docker-backend-1",
    "errorlens-backend": "docker-backend-1",
    "errorlens-nginx": "docker-nginx-1",
    "errorlens-generator": "docker-generator-1",
    "errorlens-notification": "docker-notification-worker-1",
    "errorlens-automation": "docker-automation-worker-1",
    "errorlens-collab": "docker-collab-1",
    "errorlens-redis": "docker-redis-1",
    "errorlens-postgres": "docker-postgres-1",
    "errorlens-minio": "docker-minio-1",
    "errorlens-gitlab": "errorlens-gitlab",
    "errorlens-gitlab-runner": "errorlens-gitlab-runner",
}


def get_services_status() -> list[dict]:
    """Get status of all key homelab services.

    Returns a list of services with name, port, is_up, response_time_ms, and last_checked.
    Checks both Docker containers and non-Docker services like Ollama.
    """
    results = []
    for name, info in SERVICES.items():
        entry = {
            "name": name,
            "port": info["port"],
            "is_up": False,
            "response_time_ms": None,
            "last_checked": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        url = info.get("health")
        container_name = info.get("container")
        if url:
            try:
                start = time.monotonic()
                resp = httpx.get(url, timeout=5)
                elapsed = (time.monotonic() - start) * 1000
                entry["is_up"] = resp.status_code < 500
                entry["response_time_ms"] = round(elapsed, 1)
            except Exception:
                entry["is_up"] = False
        elif container_name:
            # No port/health URL — check Docker container health status
            try:
                result = subprocess.run(
                    ["docker", "inspect", "--format", "{{.State.Status}}:{{.State.Health.Status}}", container_name],
                    capture_output=True, text=True, timeout=5
                )
                output = result.stdout.strip()
                state, _, health = output.partition(":")
                entry["is_up"] = state == "running"
                if health and health != "<no value>":
                    entry["docker_health"] = health
                    if health == "unhealthy":
                        entry["is_up"] = False
            except Exception:
                entry["is_up"] = False
        elif info["port"]:
            # No health URL — check if port is open
            import socket
            try:
                start = time.monotonic()
                s = socket.create_connection(("localhost", info["port"]), timeout=3)
                s.close()
                elapsed = (time.monotonic() - start) * 1000
                entry["is_up"] = True
                entry["response_time_ms"] = round(elapsed, 1)
            except Exception:
                entry["is_up"] = False
        results.append(entry)
    return results


def get_service_logs(service: str, lines: int = 100) -> str:
    """Get last N lines of logs for a Docker container.

    Args:
        service: Container name or alias (e.g. 'locoll', 'warehouse-api', 'errorlens',
                 'docker-backend-1'). Supports fuzzy matching — if exact name not found,
                 searches running containers for partial match.
        lines: Number of log lines to return (default 100)
    """
    container = _resolve_container(service)
    result = subprocess.run(
        ["docker", "logs", "--tail", str(lines), container],
        capture_output=True, text=True, timeout=15
    )
    output = result.stdout + result.stderr
    if not output.strip():
        return f"No logs found for container '{container}'"
    return output


def _resolve_container(service: str) -> str:
    """Resolve service name to actual container name.

    Priority: exact alias → exact docker name → fuzzy match on running containers.
    """
    # 1. Check aliases
    if service in CONTAINER_ALIASES:
        return CONTAINER_ALIASES[service]

    # 2. Try as-is (might be exact container name like 'docker-backend-1')
    check = subprocess.run(
        ["docker", "inspect", "--format", "{{.Name}}", service],
        capture_output=True, text=True, timeout=5
    )
    if check.returncode == 0:
        return service

    # 3. Fuzzy match: search running containers for partial name match
    ps = subprocess.run(
        ["docker", "ps", "--format", "{{.Names}}"],
        capture_output=True, text=True, timeout=5
    )
    if ps.returncode == 0:
        names = ps.stdout.strip().split("\n")
        # Try substring match
        matches = [n for n in names if service.lower() in n.lower()]
        if len(matches) == 1:
            return matches[0]
        if len(matches) > 1:
            # Prefer exact suffix match (e.g. "backend" → "locoll-backend-1" over "docker-backend-1")
            # Priority: name ends with "-{service}-N" pattern
            exact = [n for n in matches if n.lower().endswith(f"-{service.lower()}-1")]
            if len(exact) == 1:
                return exact[0]
            # Fallback: prefer shorter name (more specific match)
            return min(matches, key=len)

    return service  # fallback to original


def get_system_metrics() -> dict:
    """Get current server system metrics: CPU, RAM, disk, load average, uptime."""
    import psutil

    cpu_pct = psutil.cpu_percent(interval=1)
    mem = psutil.virtual_memory()
    disk = psutil.disk_usage("/")
    load = psutil.getloadavg()
    uptime_seconds = time.time() - psutil.boot_time()

    days = int(uptime_seconds // 86400)
    hours = int((uptime_seconds % 86400) // 3600)

    return {
        "cpu_percent": cpu_pct,
        "ram_used_mb": round(mem.used / 1024 / 1024),
        "ram_total_mb": round(mem.total / 1024 / 1024),
        "ram_percent": mem.percent,
        "disk_used_gb": round(disk.used / 1024 / 1024 / 1024, 1),
        "disk_total_gb": round(disk.total / 1024 / 1024 / 1024, 1),
        "disk_percent": disk.percent,
        "load_avg_1m": round(load[0], 2),
        "load_avg_5m": round(load[1], 2),
        "load_avg_15m": round(load[2], 2),
        "uptime": f"{days}d {hours}h",
    }
