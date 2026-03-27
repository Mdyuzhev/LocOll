# internal/ — Go бэкенд LocOll

## Структура

- `handlers/` — HTTP handlers (API endpoints)
- `docker/` — работа с Docker daemon
- `collector/` — сбор метрик
- `sse/` — Server-Sent Events для real-time обновлений
- `store/` — SQLite хранилище
- `system/` — системные метрики
- `ollama/` — интеграция с Ollama
- `pty/` — terminal (xterm.js backend)

## Важные правила

При добавлении нового API endpoint:
1. Создать handler в `handlers/`
2. Зарегистрировать route в `cmd/main.go` или `cmd/routes.go`
3. Если endpoint нужен фронтенду — добавить в `frontend/`

При изменении docker-интеграции (`internal/docker/`):
- Тестировать через `run_health_check(project="locoll")`
- Убедиться что `get_docker_ps()` возвращает корректные данные

## Деплой после изменений Go кода

CI/CD (GitHub Actions) деплоит автоматически при push в main.
Ручной деплой:
```python
deploy_project("locoll", build=True)
notify_deploy("locoll")
```
