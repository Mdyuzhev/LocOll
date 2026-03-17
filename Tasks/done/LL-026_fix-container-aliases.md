# LL-026: fix-container-aliases — исправить CONTAINER_ALIASES для errorlens

> Статус: Готово к разработке
> Дата создания: 2026-03-17

---

## Суть проблемы

В `tools/services.py` словарь `CONTAINER_ALIASES` для errorlens-контейнеров
содержит старые имена с префиксом `docker-*`:

```python
# Текущее состояние — НЕВЕРНО
"errorlens":           "docker-backend-1",
"errorlens-backend":   "docker-backend-1",
"errorlens-nginx":     "docker-nginx-1",
"errorlens-generator": "docker-generator-1",
...
```

После добавления `name: errorlens` в `docker/docker-compose.yml` проекта
ErrorLens все контейнеры получили префикс `errorlens-`:

```
errorlens-backend-1
errorlens-nginx-1
errorlens-pgbouncer-1
errorlens-postgres-1
errorlens-redis-1
errorlens-minio-1
errorlens-generator-1
errorlens-notification-worker-1
errorlens-automation-worker-1
errorlens-collab-1
```

Из-за этого `get_service_logs("errorlens")`, `restart_service("errorlens-backend")`
и другие инструменты с fuzzy-match находят не тот контейнер или падают с ошибкой.
Агент обнаружил и исправил это в `workflows.py` (LL-025), но `CONTAINER_ALIASES`
в `services.py` не был обновлён — это основной словарь который используется
во всех инструментах.

---

## Что изменить в tools/services.py

### CONTAINER_ALIASES — обновить errorlens-секцию

```python
# БЫЛО:
"errorlens":               "docker-backend-1",
"errorlens-backend":       "docker-backend-1",
"errorlens-nginx":         "docker-nginx-1",
"errorlens-generator":     "docker-generator-1",
"errorlens-notification":  "docker-notification-worker-1",
"errorlens-automation":    "docker-automation-worker-1",
"errorlens-collab":        "docker-collab-1",
"errorlens-redis":         "docker-redis-1",
"errorlens-postgres":      "docker-postgres-1",
"errorlens-minio":         "docker-minio-1",

# СТАЛО:
"errorlens":               "errorlens-backend-1",
"errorlens-backend":       "errorlens-backend-1",
"errorlens-nginx":         "errorlens-nginx-1",
"errorlens-generator":     "errorlens-generator-1",
"errorlens-notification":  "errorlens-notification-worker-1",
"errorlens-automation":    "errorlens-automation-worker-1",
"errorlens-collab":        "errorlens-collab-1",
"errorlens-redis":         "errorlens-redis-1",
"errorlens-postgres":      "errorlens-postgres-1",
"errorlens-minio":         "errorlens-minio-1",
"errorlens-pgbouncer":     "errorlens-pgbouncer-1",
```

Добавлен `errorlens-pgbouncer` — контейнер существует (pgbouncer-1) но
алиаса не было.

### SERVICES — обновить поля container для errorlens-воркеров

```python
# БЫЛО:
"errorlens-generator":    {"port": None, "health": None, "container": "docker-generator-1"},
"errorlens-notification": {"port": None, "health": None, "container": "docker-notification-worker-1"},
"errorlens-automation":   {"port": None, "health": None, "container": "docker-automation-worker-1"},

# СТАЛО:
"errorlens-generator":    {"port": None, "health": None, "container": "errorlens-generator-1"},
"errorlens-notification": {"port": None, "health": None, "container": "errorlens-notification-worker-1"},
"errorlens-automation":   {"port": None, "health": None, "container": "errorlens-automation-worker-1"},
```

---

## Как проверить актуальные имена контейнеров перед правкой

Перед внесением изменений убедиться в реальных именах через MCP:

```python
# Через mcp__homelab__run_shell_command:
docker ps --format "{{.Names}}" | grep -E "^(errorlens|docker)-" | sort
```

Если вывод содержит `docker-backend-1` — старые имена ещё актуальны и
менять не надо. Если `errorlens-backend-1` — правка нужна.

---

## Деплой

Изменения только в Python-файле на сервере.
Пересобрать контейнер:

```bash
cd /opt/homelab-mcp && docker compose -f docker-compose.mcp.yml up -d --build
```

---

## Критерии готовности

| Проверка | Как убедиться |
|---|---|
| Реальные имена проверены | `docker ps | grep errorlens` показывает `errorlens-*` |
| CONTAINER_ALIASES обновлён | В коде нет `docker-backend-1` в errorlens-секции |
| SERVICES обновлён | container fields воркеров содержат `errorlens-*` |
| get_service_logs работает | `get_service_logs("errorlens")` → container_resolved = `errorlens-backend-1` |
| restart_service работает | `restart_service("errorlens-backend")` → container = `errorlens-backend-1` |
| get_services_status | errorlens-воркеры корректно определяют health через docker inspect |

---

## Запрещено

- Менять имена если `docker ps` показывает что старые имена (`docker-*`)
  ещё используются — сначала проверить реальное состояние
- Удалять fuzzy-match логику в `_resolve_container` — она нужна для случаев
  когда алиаса нет
