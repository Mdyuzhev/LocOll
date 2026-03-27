# CLAUDE.md — Агент проекта LocOll (Lab Portal)

## Начало работы

При открытии нового чата — запустить `/init`.
Для полного среза сервера — `/server-status`.
Для деплоя портала или homelab-mcp — `/deploy`.

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

## Обнаружение проектов

/scan            — разовое сканирование E:\ на новые проекты
/loop 6h /scan   — сканирование каждые 6 часов
/onboard <Name>  — подключить проект под контроль LocOll

---

## 🔀 Мультиагентный режим

Если задача допускает параллелизм — **используй мультиагентный режим**.

Когда применять:
- Изменения в нескольких независимых файлах/модулях
- Обновление конфигов в нескольких проектах одновременно
- Параллельные проверки (тесты, lint, health-check)
- Исследование кодовой базы по нескольким направлениям

Когда НЕ применять:
- Шаги зависят друг от друга (результат одного нужен для следующего)
- Работа с одним файлом
- Простые линейные задачи

Принцип: максимум параллельных агентов при независимых подзадачах, строгая последовательность при зависимостях.

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
- **Репо**: `https://github.com/Mdyuzhev/homelab-mcp` (private)
- **Локальная копия**: `E:\LocOll\mcp-server\` (отдельный git, не трекается LocOll)
- **На сервере**: `/opt/homelab-mcp`
- **Деплой**: `/deploy` → [2], или `deploy_project("homelab-mcp")`
- **URL**: `http://192.168.1.74:8765/mcp`
- Python + FastMCP 3.1.1, Docker, `stateless_http=True`
- 27 инструментов, SQLite кеш (`/data/homelab.db`, named volume)

### agent-context (на сервере)
- **URL**: `http://192.168.1.74:8766/mcp`
- Python + FastMCP, Docker (python:3.12-alpine, `restart: unless-stopped`)
- Путь: `/opt/agent-context/`, данные: `/opt/agent-context/data/`
- `stateless_http=True` + `_active_state` в SQLite
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

- **CI runner**: GitHub Actions деплоит только портал (docker-compose.yml).
  homelab-mcp пересобирать через `/deploy` → выбрать [2].
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
| LL-028 | agent-teams-research | ✅ выполнена |
| LL-029 | complete-container-aliases | ✅ выполнена |
| LL-030 | update-project-claude-md | ✅ выполнена |
| LL-031 | locoll-deploy-command | ✅ выполнена |
| LL-032 | agent-context-to-server | ✅ выполнена |
| LL-034 | production-monitor | ✅ выполнена |
| LL-035 | deploy-handoff | ✅ выполнена |
| LL-036 | telegram-production-alerts | ✅ выполнена |
| LL-037 | project-autodiscovery | ✅ выполнена |
| LL-038 | project-onboarding | ✅ выполнена |

Файлы задач: `E:\LocOll\Tasks\backlog\LL-NNN_slug.md` (в работе), `E:\LocOll\Tasks\done\LL-NNN_slug.md` (выполненные)

---

## 🚫 Запрещено

- PostgreSQL, Redis — только SQLite
- npm build на сервере — только Docker multi-stage
- Хардкодить список контейнеров — только через Docker label `com.docker.compose.project`
- Тянуть новые модели Ollama — использовать только те что загружено

---

*Последнее обновление: 2026-03-18 (LL-034..038 — production monitor, deploy handoff, telegram alerts, project discovery, onboarding)*
