# LL-027: git-inspection-tools — инструменты для работы с git на сервере

> Статус: Готово к разработке
> Дата создания: 2026-03-17

---

## Цель

Добавить два инструмента для инспекции git-состояния проектов на сервере.
Сейчас агент вынужден делать `run_shell_command("cd /opt/locoll && git log --oneline -5")`
и самостоятельно парсить текстовый вывод. Структурированные инструменты
дают агенту немедленный ответ на вопросы "что сейчас задеплоено?" и
"что изменилось с последнего деплоя?" без ручного написания shell-команд.

---

## Новый файл: tools/git_tools.py

```python
"""Git inspection tools for server-side repositories."""

import subprocess
import shlex


# Маппинг project → путь репозитория на сервере
# Совпадает с PROJECT_CONFIG в workflows.py — менять синхронно
GIT_REPOS = {
    "locoll":      "/opt/locoll",
    "moex":        "/home/flomaster/moex-bot",
    "errorlens":   "/opt/errorlens",
    "warehouse":   "/opt/warehouse",
    "homelab-mcp": "/opt/homelab-mcp",
    "rag-qa":      "/home/flomaster/rag-qa",
}


def git_status(project: str) -> dict:
    """Get current git status of a deployed project.

    Shows: current branch, last commit (hash + message + author + date),
    whether working tree is clean, and any uncommitted changes.
    Use to verify what's actually deployed on the server.

    Args:
        project: Project name: 'locoll', 'moex', 'errorlens',
                 'warehouse', 'homelab-mcp', 'rag-qa'.
    """
    if project not in GIT_REPOS:
        return {
            "success": False,
            "error": f"Unknown project '{project}'. Known: {list(GIT_REPOS.keys())}",
        }

    repo = GIT_REPOS[project]

    def run(cmd):
        r = subprocess.run(
            f"cd {shlex.quote(repo)} && {cmd}",
            shell=True, capture_output=True, text=True, timeout=15
        )
        return r.stdout.strip(), r.returncode

    # Текущая ветка
    branch, _ = run("git rev-parse --abbrev-ref HEAD")

    # Последний коммит — структурировано
    log_out, rc = run(
        'git log -1 --format="%H|%h|%s|%an|%ae|%ai"'
    )
    if rc != 0 or not log_out:
        return {"success": False, "error": f"git log failed in {repo}"}

    parts = log_out.split("|", 5)
    last_commit = {
        "hash":       parts[0] if len(parts) > 0 else "",
        "short_hash": parts[1] if len(parts) > 1 else "",
        "message":    parts[2] if len(parts) > 2 else "",
        "author":     parts[3] if len(parts) > 3 else "",
        "email":      parts[4] if len(parts) > 4 else "",
        "date":       parts[5] if len(parts) > 5 else "",
    }

    # Статус рабочего дерева
    status_out, _ = run("git status --porcelain")
    is_clean = status_out == ""
    changes = status_out.splitlines() if status_out else []

    # Сколько коммитов впереди/позади origin
    ahead_behind, _ = run(
        f"git rev-list --left-right --count origin/{branch}...HEAD 2>/dev/null"
    )
    behind, ahead = 0, 0
    if ahead_behind:
        parts_ab = ahead_behind.split()
        if len(parts_ab) == 2:
            behind, ahead = int(parts_ab[0]), int(parts_ab[1])

    return {
        "project":     project,
        "repo_path":   repo,
        "branch":      branch,
        "last_commit": last_commit,
        "is_clean":    is_clean,
        "changes":     changes[:20],    # не больше 20 строк изменений
        "ahead":       ahead,           # коммитов впереди origin
        "behind":      behind,          # коммитов позади origin
    }


def git_log(project: str, n: int = 5) -> list[dict]:
    """Get last N commits of a deployed project.

    Useful to understand what was deployed recently and correlate
    with issues or container events.

    Args:
        project: Project name (see git_status for full list).
        n:       Number of commits to return (default 5, max 20).
    """
    if project not in GIT_REPOS:
        return [{"error": f"Unknown project '{project}'. Known: {list(GIT_REPOS.keys())}"}]

    repo = GIT_REPOS[project]
    n = min(n, 20)

    result = subprocess.run(
        f"cd {shlex.quote(repo)} && "
        f'git log -{n} --format="%H|%h|%s|%an|%ai|%D"',
        shell=True, capture_output=True, text=True, timeout=15
    )

    commits = []
    for line in result.stdout.strip().splitlines():
        if not line:
            continue
        parts = line.split("|", 5)
        commits.append({
            "hash":       parts[0] if len(parts) > 0 else "",
            "short_hash": parts[1] if len(parts) > 1 else "",
            "message":    parts[2] if len(parts) > 2 else "",
            "author":     parts[3] if len(parts) > 3 else "",
            "date":       parts[4] if len(parts) > 4 else "",
            "refs":       parts[5] if len(parts) > 5 else "",  # HEAD, tags
        })

    return commits
```

---

## Регистрация в server.py

```python
from tools.git_tools import git_status, git_log

mcp.tool()(git_status)
mcp.tool()(git_log)
```

---

## Деплой

```bash
cd /opt/homelab-mcp && docker compose -f docker-compose.mcp.yml up -d --build
```

---

## Почему эти инструменты важны

Сейчас агент не имеет надёжного способа ответить на вопрос "что сейчас
задеплоено на сервере?" без ручной shell-команды. `git_status` закрывает
этот пробел: один вызов возвращает ветку, последний коммит, состояние
рабочего дерева и информацию об опережении/отставании от origin.

`git_log` позволяет агенту быстро соотнести события контейнеров (из `get_events`)
с конкретными коммитами — например "unhealthy после коммита abc123 в 14:30".

---

## Критерии готовности

| Проверка | Как убедиться |
|---|---|
| git_status("locoll") | Возвращает branch, last_commit с hash и message, is_clean |
| git_log("moex", n=3) | Возвращает список из 3 коммитов со структурированными полями |
| Неизвестный проект | git_status("unknown") → понятная ошибка со списком known |
| ahead/behind | Показывает 0/0 для синхронизированного репо |

---

## Запрещено

- Делать `git fetch` или `git pull` внутри инструментов — только чтение, без изменений
- Возвращать сырой текст `git log` — только структурированный JSON
- Ставить `n` больше 20 — защита от огромных ответов
