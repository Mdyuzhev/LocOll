"""Управление процессом MOEX-бота."""
import subprocess
import os
import signal
import time

BOT_DIR = "/home/flomaster/moex-bot"
PID_FILE = f"{BOT_DIR}/bot.pid"


def manage_bot(action: str) -> dict:
    """Управление MOEX-ботом: start, stop, restart, status.

    Args:
        action: Действие — 'start', 'stop', 'restart', 'status'
    """
    if action == "status":
        return _status()
    elif action == "stop":
        return _stop()
    elif action == "start":
        return _start()
    elif action == "restart":
        stopped = _stop()
        started = _start()
        return {"action": "restart", "stop": stopped, "start": started}
    else:
        return {"error": f"Unknown action: {action}. Use: start/stop/restart/status"}


def _get_pid():
    if os.path.exists(PID_FILE):
        pid = int(open(PID_FILE).read().strip())
        try:
            os.kill(pid, 0)
            return pid
        except ProcessLookupError:
            os.remove(PID_FILE)
    return None


def _status():
    pid = _get_pid()
    if pid:
        return {"running": True, "pid": pid}
    return {"running": False, "pid": None}


def _stop():
    pid = _get_pid()
    if not pid:
        return {"stopped": False, "reason": "not running"}
    try:
        os.kill(pid, signal.SIGTERM)
        for _ in range(10):
            time.sleep(0.5)
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                if os.path.exists(PID_FILE):
                    os.remove(PID_FILE)
                return {"stopped": True, "pid": pid}
        os.kill(pid, signal.SIGKILL)
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
        return {"stopped": True, "pid": pid, "forced": True}
    except Exception as e:
        return {"stopped": False, "error": str(e)}


def _start():
    if _get_pid():
        return {"started": False, "reason": "already running", "pid": _get_pid()}
    try:
        os.makedirs(f"{BOT_DIR}/logs", exist_ok=True)
        proc = subprocess.Popen(
            ["python3", "-m", "src.main"],
            cwd=BOT_DIR,
            stdout=open(f"{BOT_DIR}/logs/startup.log", "w"),
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        with open(PID_FILE, "w") as f:
            f.write(str(proc.pid))
        return {"started": True, "pid": proc.pid}
    except Exception as e:
        return {"started": False, "error": str(e)}
