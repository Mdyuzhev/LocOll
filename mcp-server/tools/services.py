"""Tools for service status, logs, and system metrics."""

import subprocess
import time
import httpx


SERVICES = {
    # --- LocOll Portal ---
    "locoll-backend": {"port": 8010, "health": "http://localhost:8010/api/v1/health"},
    "locoll-nginx":   {"port": 4000, "health": "http://localhost:4000/"},

    # --- Warehouse ---
    "warehouse-api":           {"port": 8080, "health": "http://localhost:8080/health"},
    "warehouse-grafana":       {"port": 3001, "health": "http://localhost:3001/"},
    "warehouse-prometheus":    {"port": 9090, "health": "http://localhost:9090/-/healthy"},
    "warehouse-frontend":      {"port": 80,   "health": "http://localhost:80/"},
    "warehouse-telegram-bot":  {"port": 8000, "health": None},
    "warehouse-robot":         {"port": 8070, "health": "http://localhost:8070/health"},
    "warehouse-analytics":     {"port": 8090, "health": None},
    "warehouse-uplink-bot":    {"port": 8001, "health": None},

    # --- ErrorLens ---
    "errorlens-backend":      {"port": 8002, "health": "http://localhost:8002/health"},
    "errorlens-nginx":        {"port": 3000, "health": "http://localhost:3000/"},
    "errorlens-generator":    {"port": None, "health": None, "container": "errorlens-generator-1"},
    "errorlens-notification": {"port": None, "health": None, "container": "errorlens-notification-worker-1"},
    "errorlens-automation":   {"port": None, "health": None, "container": "errorlens-automation-worker-1"},

    # --- moex ---
    "moex-bot":      {"port": None, "health": None, "container": "moex-bot"},
    "moex-postgres": {"port": 5434, "health": None, "container": "moex-postgres"},

    # --- homelab-mcp ---
    "homelab-mcp": {"port": 8765, "health": "http://localhost:8765/health"},

    # --- Scout ---
    "scout-mcp":      {"port": 8020, "health": "http://localhost:8020/health"},
    "scout-postgres": {"port": 5436, "health": None, "container": "scout-postgres"},

    # --- RAG QA ---
    "rag-qa": {"port": 8001, "health": "http://localhost:8001/health", "container": "rag_qa"},

    # --- Инфраструктура ---
    "ollama": {"port": 11434, "health": "http://localhost:11434/"},
}

# Map short names to actual docker container names (verified via docker ps)
CONTAINER_ALIASES = {
    # --- LocOll ---
    "locoll":         "locoll-backend-1",
    "locoll-backend": "locoll-backend-1",
    "locoll-nginx":   "locoll-nginx-1",

    # --- Warehouse ---
    "warehouse":              "warehouse-api-1",
    "warehouse-api":          "warehouse-api-1",
    "warehouse-grafana":      "warehouse-grafana-1",
    "warehouse-prometheus":   "warehouse-prometheus-1",
    "warehouse-frontend":     "warehouse-frontend-1",
    "warehouse-telegram-bot": "warehouse-telegram-bot-1",
    "warehouse-robot":        "warehouse-robot-1",
    "warehouse-analytics":    "warehouse-analytics-1",
    "warehouse-uplink-bot":   "warehouse-uplink-bot-1",
    "warehouse-redis":        "warehouse-redis-1",
    "warehouse-kafka":        "warehouse-kafka-1",
    "warehouse-db":           "warehouse-db-1",
    "warehouse-pgbouncer":    "warehouse-pgbouncer-1",

    # --- ErrorLens ---
    "errorlens":              "errorlens-backend-1",
    "errorlens-backend":      "errorlens-backend-1",
    "errorlens-nginx":        "errorlens-nginx-1",
    "errorlens-generator":    "errorlens-generator-1",
    "errorlens-notification": "errorlens-notification-worker-1",
    "errorlens-automation":   "errorlens-automation-worker-1",
    "errorlens-collab":       "errorlens-collab-1",
    "errorlens-redis":        "errorlens-redis-1",
    "errorlens-postgres":     "errorlens-postgres-1",
    "errorlens-minio":        "errorlens-minio-1",
    "errorlens-pgbouncer":    "errorlens-pgbouncer-1",

    # --- ErrorLens Monitoring ---
    "errorlens-grafana":    "errorlens-monitoring-grafana-1",
    "errorlens-loki":       "errorlens-monitoring-loki-1",
    "errorlens-prometheus": "errorlens-monitoring-prometheus-1",
    "errorlens-promtail":   "errorlens-monitoring-promtail-1",

    # --- moex ---
    "moex":          "moex-bot",
    "moex-bot":      "moex-bot",
    "moex-postgres": "moex-postgres",

    # --- homelab-mcp ---
    "homelab-mcp": "homelab-mcp",

    # --- Scout ---
    "scout":          "scout-mcp",
    "scout-mcp":      "scout-mcp",
    "scout-postgres": "scout-postgres",

    # --- RAG QA ---
    "rag-qa":  "rag_qa",
    "rag_qa":  "rag_qa",
    "ragqa":   "rag_qa",

    # --- Инфраструктура ---
    "ollama":       "ollama",
    "vpn-proxy":    "vpn-proxy",

    # --- Telegram боты ---
    "homelab-bot":  "homelab-bot-homelab-bot-1",
    "telegram-bot": "homelab-bot-homelab-bot-1",
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
    """Get current server system metrics: CPU, RAM, disk, load average, uptime.

    Results cached for 30 seconds. Cached responses include from_cache: true.
    """
    from .cache import get_cached_metrics, set_cached_metrics

    cached = get_cached_metrics()
    if cached:
        cached["from_cache"] = True
        return cached

    import psutil

    cpu_pct = psutil.cpu_percent(interval=0.5)
    mem = psutil.virtual_memory()
    disk = psutil.disk_usage("/")
    load = psutil.getloadavg()
    uptime_seconds = time.time() - psutil.boot_time()

    days = int(uptime_seconds // 86400)
    hours = int((uptime_seconds % 86400) // 3600)

    result = {
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
    set_cached_metrics(result)
    return result
