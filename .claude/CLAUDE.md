# CLAUDE.md — Агент проекта LocOll (Lab Portal)

## Начало работы

При открытии нового чата — запустить `/init`.
Для полного среза сервера — `/server-status`.

## 🔄 ЧЕКПОИНТЫ — обязательно

Контекст разговора конечен и сжимается без предупреждения. Чтобы не терять прогресс:

1. **При завершении каждого todo** — вызвать `mcp__agent-context__checkpoint` с описанием что сделано.
2. **Каждые 5 вызовов инструментов** — принудительный checkpoint.
3. **При получении system-reminder о сжатии контекста** — немедленный checkpoint.
4. **Перед завершением задачи** — `end_session` с полным итогом.

Формат checkpoint: `"[шаг N] краткое описание — что сделано, что дальше"`

Если `agent-context` MCP недоступен — предупредить и продолжить без него.

---

## Производственный мониторинг

/status — разовый производственный срез (все проекты, 24 часа)
/loop 30m /status — автоматический мониторинг каждые 30 минут

---

## Что это за проект

**LocOll** — экспериментальный homelab-портал. Единый дашборд для управления и
мониторинга домашней лаборатории: контейнеры, метрики, логи, аналитика, терминал, AI.

Проект **экспериментальный** — полигон для изучения Go, WebAssembly, htmx, SSE, DuckDB WASM.

**Владелец**: Flomaster (Михаил)
**GitHub**: `https://github.com/Mdyuzhev/LocOll`

---

## Подключение к серверу

Правила подключения (paramiko, адреса, креды) — в глобальном окружении (`start_session`).
Путь проекта на сервере: `/opt/locoll`
Ключевые порты: Portal **4000**, backend **8010**, homelab-mcp **8765**, Ollama **11434**.

---

## MCP-архитектура (текущее состояние)

### homelab-mcp (на сервере)
- **URL**: `http://192.168.1.74:8765/mcp`
- Python + FastMCP 3.1.1, Docker, `stateless_http=True`
- 23 инструмента (включая `git_status`, `git_log` из LL-027)
- SQLite кеш: `/data/homelab.db` (named volume), metrics TTL=30s, docker_ps TTL=15s, events retention=7d
- `/health` endpoint не работает в FastMCP 3.1.1 — проверять через MCP initialize

### agent-context (локально, Docker Desktop)
- **URL**: `http://127.0.0.1:8766/mcp`
- Python + FastMCP, Docker Desktop (python:3.12-alpine, `restart: unless-stopped`)
- `stateless_http=True` + `_active_state` в SQLite
- Данные: `E:\agent-context\data\` (volume mount → /data)
- 8 инструментов, `start_session` возвращает полный контекст (LL-018):
  метрики, контейнеры, события, заметки — параллельно через `asyncio.gather`
- `get_context` — лёгкий, только локальные данные из SQLite

### /init — один вызов
`start_session` = единственная точка входа. Health check, метрики, docker ps,
events, notes — всё параллельно внутри. Не нужен отдельный curl или get_context.

---

Полный справочник (стек, архитектура, структура, команды): `.claude/reference.md`

---

## ⚠️ Известные проблемы

- **CI runner**: GitHub Actions деплоит только портал (docker-compose.yml). homelab-mcp (docker-compose.mcp.yml) пересобирать вручную на сервере.
- **Go WASM toolchain**: Docker скачивает Go 1.25 toolchain — увеличивает время сборки.

---

## 📋 Реестр задач

| ID | Название | Статус |
|----|----------|--------|
| LL-001 | portal-foundation | ✅ выполнена |
| LL-002 | duckdb-fix-and-modals | ✅ выполнена |
| LL-003 | mcp-server-homelab | ✅ выполнена |
| LL-004 | agent-context-mcp | ✅ выполнена |
| LL-005 | server-mcp-bridge | ✅ выполнена |
| LL-006 | homelab-telegram-bot | ✅ выполнена |
| LL-007 | telegram-notes-to-agent | ✅ выполнена |
| LL-008 | claude-bridge-endpoint | ✅ выполнена |
| LL-009 | portal-improvements | ✅ выполнена |
| LL-010 | homelab-proxy-stability | ✅ выполнена |
| LL-011 | init-speed-optimization | ✅ выполнена |
| LL-012 | startup-speed | ✅ выполнена |
| LL-013 | agent-context-http | ✅ выполнена |
| LL-014 | pm2-windows-autostart | ✅ выполнена |
| LL-015 | homelab-mcp-stateless | ✅ выполнена |
| LL-016 | agent-context-python | ✅ выполнена |
| LL-017 | homelab-mcp-sqlite | ✅ выполнена |
| LL-018 | smart-start-session | ✅ выполнена |
| LL-019 | resolver-active-state-sqlite | ❌ неактуальна (Python уже использует SQLite) |
| LL-020 | db-commit-safety | ✅ выполнена |
| LL-021 | shared-conventions | ✅ выполнена |
| LL-022 | docker-stats-tool | ✅ выполнена |
| LL-023 | mcp-tool-improvements | ✅ выполнена |
| LL-024 | agent-context-per-session-state | ✅ выполнена |
| LL-025 | workflow-tools | ✅ выполнена |
| LL-026 | fix-container-aliases | ✅ выполнена |
| LL-027 | git-inspection-tools | ✅ выполнена |
| LL-034 | production-monitor | ✅ выполнена |

Файлы задач: `E:\LocOll\Tasks\backlog\LL-NNN_slug.md` (в работе), `E:\LocOll\Tasks\done\LL-NNN_slug.md` (выполненные)

---

## 🚫 Запрещено

- PostgreSQL, Redis — только SQLite
- npm build на сервере — только Docker multi-stage
- Хардкодить список контейнеров — только через Docker label `com.docker.compose.project`
- Тянуть новые модели Ollama — использовать только те что загружено

---

*Последнее обновление: 2026-03-17 (LL-027 — git inspection tools)*
