"""Shell and deployment tools."""

import subprocess
import shlex


ALLOWED_COMPOSE_DIRS = [
    "/opt/locoll",
    "/home/flomaster/projects/locoll",
    "/home/flomaster/docker",
    "/home/flomaster/warehouse",
    "/opt/homelab-mcp",
]


def run_shell_command(command: str, timeout_seconds: int = 60) -> dict:
    """Execute a shell command on the server.

    Use for git operations, file inspection, system commands, etc.
    Commands run as the container's root user with host network access.

    Args:
        command: Shell command to execute (e.g. 'ls -la /home/flomaster/projects')
        timeout_seconds: Max execution time in seconds (default 60, max 300)
    """
    timeout_seconds = min(timeout_seconds, 300)

    try:
        result = subprocess.run(
            command, shell=True,
            capture_output=True, text=True,
            timeout=timeout_seconds,
        )
        return {
            "exit_code": result.returncode,
            "stdout": result.stdout[-10000:] if len(result.stdout) > 10000 else result.stdout,
            "stderr": result.stderr[-5000:] if len(result.stderr) > 5000 else result.stderr,
        }
    except subprocess.TimeoutExpired:
        return {"exit_code": -1, "stdout": "", "stderr": f"Command timed out after {timeout_seconds}s"}
    except Exception as e:
        return {"exit_code": -1, "stdout": "", "stderr": str(e)}


def exec_in_container(container: str, command: str, timeout_seconds: int = 30) -> dict:
    """Execute a command inside a Docker container.

    Args:
        container: Container name (e.g. 'warehouse-api', 'docker-backend-1')
        command: Command to run inside the container (e.g. 'alembic current', 'cat /app/.env')
        timeout_seconds: Max execution time (default 30, max 120)
    """
    from .services import _resolve_container

    container = _resolve_container(container)
    timeout_seconds = min(timeout_seconds, 120)

    try:
        result = subprocess.run(
            ["docker", "exec", container, "sh", "-c", command],
            capture_output=True, text=True,
            timeout=timeout_seconds,
        )
        return {
            "container": container,
            "exit_code": result.returncode,
            "stdout": result.stdout[-10000:] if len(result.stdout) > 10000 else result.stdout,
            "stderr": result.stderr[-5000:] if len(result.stderr) > 5000 else result.stderr,
        }
    except subprocess.TimeoutExpired:
        return {"container": container, "exit_code": -1, "stdout": "", "stderr": f"Timed out after {timeout_seconds}s"}
    except Exception as e:
        return {"container": container, "exit_code": -1, "stdout": "", "stderr": str(e)}


def grep_docker_logs(container: str, pattern: str, lines: int = 500) -> dict:
    """Search Docker container logs for a pattern (grep).

    Args:
        container: Container name (e.g. 'warehouse-api', 'docker-backend-1')
        pattern: Grep pattern to search for (e.g. 'ERROR', 'traceback', '500')
        lines: Number of recent log lines to search through (default 500)
    """
    from .services import _resolve_container

    container = _resolve_container(container)

    try:
        logs = subprocess.run(
            ["docker", "logs", "--tail", str(lines), container],
            capture_output=True, text=True, timeout=15
        )
        all_logs = logs.stdout + logs.stderr

        grep = subprocess.run(
            ["grep", "-i", "-n", pattern],
            input=all_logs, capture_output=True, text=True, timeout=10
        )
        matches = grep.stdout.strip()
        match_count = len(matches.split("\n")) if matches else 0

        return {
            "container": container,
            "pattern": pattern,
            "lines_searched": lines,
            "match_count": match_count,
            "matches": matches[-10000:] if len(matches) > 10000 else matches,
        }
    except Exception as e:
        return {"container": container, "pattern": pattern, "error": str(e)}


def compose_up(compose_dir: str, service: str = "", build: bool = True, timeout_seconds: int = 180) -> dict:
    """Run docker compose up (-d --build) in a directory.

    Use for deploying/redeploying services after code changes.

    Args:
        compose_dir: Directory with docker-compose.yml (e.g. '/home/flomaster/projects/locoll')
        service: Specific service to rebuild (empty = all services)
        build: Whether to rebuild images (default True)
        timeout_seconds: Max execution time (default 180, max 600)
    """
    timeout_seconds = min(timeout_seconds, 600)

    cmd = f"cd {shlex.quote(compose_dir)} && docker compose up -d"
    if build:
        cmd += " --build"
    if service:
        cmd += f" {shlex.quote(service)}"
    cmd += " 2>&1"

    try:
        result = subprocess.run(
            cmd, shell=True,
            capture_output=True, text=True,
            timeout=timeout_seconds,
        )
        return {
            "compose_dir": compose_dir,
            "service": service or "(all)",
            "build": build,
            "exit_code": result.returncode,
            "output": result.stdout[-10000:] if len(result.stdout) > 10000 else result.stdout,
        }
    except subprocess.TimeoutExpired:
        return {"compose_dir": compose_dir, "exit_code": -1, "output": f"Timed out after {timeout_seconds}s"}
    except Exception as e:
        return {"compose_dir": compose_dir, "exit_code": -1, "output": str(e)}
