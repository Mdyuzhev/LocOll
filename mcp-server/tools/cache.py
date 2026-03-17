"""SQLite cache for homelab-mcp metrics and container events."""

import sqlite3
import json
import time
import os
import threading

DB_PATH = os.environ.get("CACHE_DB_PATH", "/data/homelab.db")

# TTL settings
METRICS_TTL = 30   # seconds — system metrics
DOCKER_TTL  = 15   # seconds — container list

_lock = threading.Lock()
_conn = None


def get_db() -> sqlite3.Connection:
    """Singleton connection with WAL mode for concurrent reads."""
    global _conn
    if _conn is None:
        os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)
        _conn = sqlite3.connect(DB_PATH, check_same_thread=False)
        _conn.execute("PRAGMA journal_mode=WAL")
        _conn.execute("PRAGMA synchronous=NORMAL")
        _init_schema(_conn)
    return _conn


def _init_schema(conn):
    conn.executescript("""
        CREATE TABLE IF NOT EXISTS metrics_cache (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            data TEXT NOT NULL,
            updated_at REAL NOT NULL
        );

        CREATE TABLE IF NOT EXISTS docker_cache (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            data TEXT NOT NULL,
            updated_at REAL NOT NULL
        );

        CREATE TABLE IF NOT EXISTS container_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ts REAL NOT NULL,
            container TEXT NOT NULL,
            project TEXT,
            event TEXT NOT NULL,
            detail TEXT,
            created_at TEXT DEFAULT (datetime('now'))
        );

        CREATE INDEX IF NOT EXISTS idx_events_ts ON container_events(ts);
        CREATE INDEX IF NOT EXISTS idx_events_container ON container_events(container);
    """)
    conn.commit()


# --- Metrics cache ---

def get_cached_metrics() -> dict | None:
    """Return cached metrics if fresher than METRICS_TTL seconds."""
    row = get_db().execute(
        "SELECT data, updated_at FROM metrics_cache WHERE id = 1"
    ).fetchone()
    if row and (time.time() - row[1]) < METRICS_TTL:
        return json.loads(row[0])
    return None


def set_cached_metrics(data: dict):
    """Save metrics to cache."""
    with _lock:
        get_db().execute(
            "INSERT OR REPLACE INTO metrics_cache (id, data, updated_at) VALUES (1, ?, ?)",
            (json.dumps(data), time.time())
        )
        get_db().commit()


# --- Docker cache ---

def get_cached_docker() -> list | None:
    """Return cached container list if fresher than DOCKER_TTL seconds."""
    row = get_db().execute(
        "SELECT data, updated_at FROM docker_cache WHERE id = 1"
    ).fetchone()
    if row and (time.time() - row[1]) < DOCKER_TTL:
        return json.loads(row[0])
    return None


def set_cached_docker(data: list):
    """Save container list to cache."""
    with _lock:
        get_db().execute(
            "INSERT OR REPLACE INTO docker_cache (id, data, updated_at) VALUES (1, ?, ?)",
            (json.dumps(data), time.time())
        )
        get_db().commit()


# --- Container events ---

def record_event(container: str, event: str, project: str | None = None, detail: str | None = None):
    """Record a container event. Auto-cleans entries older than 7 days."""
    with _lock:
        db = get_db()
        db.execute(
            "INSERT INTO container_events (ts, container, project, event, detail) VALUES (?, ?, ?, ?, ?)",
            (time.time(), container, project, event, detail)
        )
        # Retention cleanup
        db.execute("DELETE FROM container_events WHERE ts < ?", (time.time() - 7 * 86400,))
        db.commit()


def get_recent_events(hours: int = 1, container: str | None = None, project: str | None = None) -> list[dict]:
    """Get events for the last N hours. Optional filter by container or project."""
    since = time.time() - hours * 3600
    query = "SELECT ts, container, project, event, detail FROM container_events WHERE ts > ?"
    params: list = [since]

    if container:
        query += " AND container = ?"
        params.append(container)
    elif project:
        query += " AND project = ?"
        params.append(project)

    query += " ORDER BY ts DESC"
    rows = get_db().execute(query, params).fetchall()

    return [
        {"ts": r[0], "container": r[1], "project": r[2], "event": r[3], "detail": r[4]}
        for r in rows
    ]
