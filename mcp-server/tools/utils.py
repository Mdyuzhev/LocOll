"""Utility tools: health check, server info."""

import subprocess
import time


def run_health_check() -> dict:
    """Run comprehensive health check of all homelab services.

    Checks all Docker containers, key services, and system resources.
    Returns overall status (OK/DEGRADED/DOWN) and list of issues.
    """
    from .services import get_services_status, get_system_metrics

    services = get_services_status()
    metrics = get_system_metrics()

    issues = []
    up_count = sum(1 for s in services if s["is_up"])
    total = len(services)

    for s in services:
        if not s["is_up"]:
            docker_health = s.get("docker_health", "")
            if docker_health:
                issues.append(f"Service '{s['name']}' is {docker_health} (Docker health check)")
            elif s["port"]:
                issues.append(f"Service '{s['name']}' (port {s['port']}) is DOWN")
            else:
                issues.append(f"Service '{s['name']}' container is not running")

    # Check resource thresholds
    if metrics["cpu_percent"] > 90:
        issues.append(f"CPU usage critical: {metrics['cpu_percent']}%")
    if metrics["ram_percent"] > 90:
        issues.append(f"RAM usage critical: {metrics['ram_percent']}%")
    if metrics["disk_percent"] > 90:
        issues.append(f"Disk usage critical: {metrics['disk_percent']}%")

    # Determine overall status
    if not issues:
        status = "OK"
    elif up_count > total * 0.5:
        status = "DEGRADED"
    else:
        status = "DOWN"

    return {
        "status": status,
        "services_up": up_count,
        "services_total": total,
        "issues": issues,
        "system": metrics,
        "checked_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }


def get_server_info() -> dict:
    """Get general server information: hostname, IPs, OS, Docker version."""
    hostname = subprocess.run(
        ["hostname"], capture_output=True, text=True, timeout=5
    ).stdout.strip()

    # Get local IP
    local_ip = subprocess.run(
        ["hostname", "-I"], capture_output=True, text=True, timeout=5
    ).stdout.strip().split()[0] if True else ""

    # Get Tailscale IP (may not be installed in container)
    try:
        ts_result = subprocess.run(
            ["tailscale", "ip", "-4"], capture_output=True, text=True, timeout=5
        )
        tailscale_ip = ts_result.stdout.strip() if ts_result.returncode == 0 else "N/A"
    except FileNotFoundError:
        tailscale_ip = "N/A (tailscale not in container)"

    # OS info
    os_info = subprocess.run(
        ["cat", "/etc/os-release"], capture_output=True, text=True, timeout=5
    )
    os_name = ""
    for line in os_info.stdout.split("\n"):
        if line.startswith("PRETTY_NAME="):
            os_name = line.split("=", 1)[1].strip('"')
            break

    kernel = subprocess.run(
        ["uname", "-r"], capture_output=True, text=True, timeout=5
    ).stdout.strip()

    docker_version = subprocess.run(
        ["docker", "version", "--format", "{{.Server.Version}}"],
        capture_output=True, text=True, timeout=5
    ).stdout.strip()

    # Ollama models
    import httpx
    try:
        resp = httpx.get("http://localhost:11434/api/tags", timeout=5)
        models = [m["name"] for m in resp.json().get("models", [])]
    except Exception:
        models = []

    return {
        "hostname": hostname,
        "local_ip": local_ip,
        "tailscale_ip": tailscale_ip,
        "os": os_name,
        "kernel": kernel,
        "docker_version": docker_version,
        "ollama_models": models,
    }
