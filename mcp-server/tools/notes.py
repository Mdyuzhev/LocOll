"""Notes storage — SQLite-backed notes for Telegram → agent-context pipeline."""

import sqlite3
import time
from datetime import datetime, timezone, timedelta

DB_PATH = "/home/flomaster/.homelab-notes.db"
MSK = timezone(timedelta(hours=3))


def _get_db() -> sqlite3.Connection:
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    conn.execute("""
        CREATE TABLE IF NOT EXISTS notes (
            id        INTEGER PRIMARY KEY AUTOINCREMENT,
            text      TEXT    NOT NULL,
            project   TEXT,
            done      INTEGER DEFAULT 0,
            created   INTEGER NOT NULL,
            done_at   INTEGER
        )
    """)
    conn.commit()
    return conn


def _human_time(ts: int) -> str:
    """Format unix timestamp as human-readable Moscow time."""
    now_msk = datetime.now(MSK)
    dt_msk = datetime.fromtimestamp(ts, MSK)
    time_str = dt_msk.strftime("%H:%M")

    if dt_msk.date() == now_msk.date():
        return f"сегодня {time_str}"
    if dt_msk.date() == (now_msk - timedelta(days=1)).date():
        return f"вчера {time_str}"
    return f"{dt_msk.strftime('%d %b')} {time_str}"


async def add_note(text: str, project: str | None = None) -> dict:
    """Add a new note.

    Args:
        text: Note text
        project: Optional project name (e.g. 'locoll', 'moex')
    """
    now = int(time.time())
    db = _get_db()
    try:
        cur = db.execute(
            "INSERT INTO notes (text, project, created) VALUES (?, ?, ?)",
            (text, project, now),
        )
        db.commit()
        return {"id": cur.lastrowid, "text": text, "created": _human_time(now)}
    finally:
        db.close()


async def list_notes(project: str | None = None, include_done: bool = False) -> list[dict]:
    """List notes. By default only pending (not done).

    Args:
        project: Filter by project name (optional)
        include_done: Include completed notes (default false)
    """
    db = _get_db()
    try:
        query = "SELECT id, text, project, done, created, done_at FROM notes"
        conditions = []
        params = []

        if not include_done:
            conditions.append("done = 0")
        if project:
            conditions.append("project = ?")
            params.append(project)

        if conditions:
            query += " WHERE " + " AND ".join(conditions)
        query += " ORDER BY created ASC"

        rows = db.execute(query, params).fetchall()
        return [
            {
                "id": r["id"],
                "text": r["text"],
                "project": r["project"],
                "done": bool(r["done"]),
                "created_human": _human_time(r["created"]),
            }
            for r in rows
        ]
    finally:
        db.close()


async def complete_note(note_id: int) -> dict:
    """Mark a note as completed.

    Args:
        note_id: ID of the note to complete
    """
    now = int(time.time())
    db = _get_db()
    try:
        cur = db.execute(
            "UPDATE notes SET done = 1, done_at = ? WHERE id = ? AND done = 0",
            (now, note_id),
        )
        db.commit()
        if cur.rowcount == 0:
            return {"ok": False, "error": "not found"}
        return {"ok": True, "id": note_id}
    finally:
        db.close()
