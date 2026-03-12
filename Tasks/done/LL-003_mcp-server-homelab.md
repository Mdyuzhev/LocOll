# LL-003 — MCP-сервер для управления homelab-сервером

## Контекст

Сейчас агент взаимодействует с сервером через Paramiko + SSH с password-based auth
(ключевая аутентификация не работает из-за Windows + кириллический username).
Каждая новая операция требует написания нового SSH-обёрточного кода прямо в задачах.

Цель — создать собственный MCP-сервер на Python (fastmcp), задеплоенный прямо на
сервер `192.168.1.74`, который экспонирует набор строго типизированных инструментов
для управления homelab. После деплоя агент подключается к MCP-серверу напрямую,
без какого-либо SSH-кода в задачах.

## Цель задачи

Разработать, задеплоить и подключить MCP-сервер `homelab-mcp`, который:

1. Работает как Docker-контейнер на сервере (добавляется в отдельный docker-compose)
2. Принимает подключения на порту `8765` (SSE transport)
3. Доступен агенту как локально (192.168.1.74:8765), так и по Tailscale (100.81.243.12:8765)
4. Экспонирует инструменты для управления всеми сервисами homelab

## Стек

- **Python 3.12** + **fastmcp** (декларативный MCP-фреймворк)
- **SSE transport** (Server-Sent Events) — нативно поддерживается Claude Code
- **Docker Compose** (отдельный файл `docker-compose.mcp.yml` рядом с основными compose-файлами)
- Размещение на сервере: `/opt/homelab-mcp/`

## Инструменты (tools) MCP-сервера

Ниже — полный список инструментов, которые нужно реализовать. Каждый инструмент
должен иметь строгую типизацию аргументов и информативные описания (они попадут
в промпт агента).

### Группа: Статус сервисов

**`get_services_status`** — возвращает статус всех ключевых сервисов:
GitLab (8080), YouTrack (8088), Grafana (3000), Prometheus (9090),
Allure (5050/5252), Docker daemon, K3s.
Для каждого сервиса: имя, порт, is_up (bool), response_time_ms, last_checked.

**`get_service_logs`(service: str, lines: int = 100)** — возвращает последние N строк
логов Docker-контейнера или systemd-юнита. Поддерживаемые значения `service`:
`gitlab`, `youtrack`, `grafana`, `prometheus`, `allure`, `errorlens`, `warehouse-api`,
`lenscheck`, `locoll`.

**`get_system_metrics`** — CPU %, RAM used/total, disk used/total по каждому разделу,
uptime сервера. Читается через `/proc/stat`, `/proc/meminfo`, `df`.

### Группа: Управление контейнерами

**`restart_service`(service: str)** — перезапускает Docker-контейнер.
Принимает те же имена, что и `get_service_logs`.
Возвращает: статус до и после перезапуска, время операции.

**`stop_service`(service: str)** — останавливает контейнер.

**`start_service`(service: str)** — запускает контейнер.

**`get_docker_ps`** — полный вывод `docker ps -a` в структурированном виде:
id, name, image, status, ports, created.

### Группа: Kubernetes / K3s

**`kubectl_get`(resource: str, namespace: str = "warehouse")** — выполняет
`kubectl get <resource> -n <namespace>` и возвращает структурированный список.
Примеры resource: `pods`, `deployments`, `services`, `ingress`.

**`kubectl_rollout_status`(deployment: str, namespace: str = "warehouse")**  
— статус rollout конкретного деплоймента.

**`kubectl_rollout_restart`(deployment: str, namespace: str = "warehouse")**  
— перезапуск деплоймента через rolling update.

**`kubectl_logs`(pod: str, namespace: str = "warehouse", lines: int = 100)**  
— логи конкретного пода.

### Группа: GitLab CI/CD

**`trigger_pipeline`(project_id: str, ref: str = "main")**  
— запускает GitLab CI/CD pipeline через API.
GitLab token читается из переменной окружения `GITLAB_TOKEN` (прописан в ~/.bashrc).
Возвращает: pipeline_id, status, web_url.

**`get_pipeline_status`(project_id: str, pipeline_id: int)**  
— статус конкретного pipeline: status, duration, jobs.

**`list_pipelines`(project_id: str, per_page: int = 5)**  
— последние N pipeline'ов проекта с их статусами.

### Группа: YouTrack

**`create_youtrack_issue`(project: str, summary: str, description: str = "")**  
— создаёт задачу в YouTrack через API.
Token читается из переменной окружения `YOUTRACK_TOKEN`.
Возвращает: issue_id, url.

**`comment_youtrack_issue`(issue_id: str, text: str)**  
— добавляет комментарий к задаче.

**`get_youtrack_issue`(issue_id: str)**  
— возвращает summary, description, state, assignee задачи.

### Группа: Утилиты

**`run_health_check`** — комплексная проверка всего homelab: опрашивает все сервисы,
K3s pods, Docker containers, возвращает единый health-report с общим статусом
(OK / DEGRADED / DOWN) и списком проблем.

**`get_server_info`** — общая информация: hostname, IP-адреса (локальный + Tailscale),
OS, kernel, Docker version, K3s version.

## Структура проекта

```
/opt/homelab-mcp/
├── server.py           # Точка входа, регистрация всех tools
├── tools/
│   ├── __init__.py
│   ├── services.py     # get_services_status, get_service_logs, get_system_metrics
│   ├── docker.py       # restart_service, stop_service, start_service, get_docker_ps
│   ├── kubernetes.py   # kubectl_* инструменты
│   ├── gitlab.py       # trigger_pipeline, get_pipeline_status, list_pipelines
│   ├── youtrack.py     # create_youtrack_issue, comment_youtrack_issue, get_youtrack_issue
│   └── utils.py        # run_health_check, get_server_info
├── requirements.txt
├── Dockerfile
└── docker-compose.mcp.yml
```

## Детали реализации

### server.py — точка входа

```python
from fastmcp import FastMCP
from tools.services import get_services_status, get_service_logs, get_system_metrics
from tools.docker import restart_service, stop_service, start_service, get_docker_ps
from tools.kubernetes import kubectl_get, kubectl_rollout_status, kubectl_rollout_restart, kubectl_logs
from tools.gitlab import trigger_pipeline, get_pipeline_status, list_pipelines
from tools.youtrack import create_youtrack_issue, comment_youtrack_issue, get_youtrack_issue
from tools.utils import run_health_check, get_server_info

mcp = FastMCP("homelab-mcp")

# Регистрируем все инструменты
mcp.tool()(get_services_status)
mcp.tool()(get_service_logs)
# ... (все остальные инструменты)

if __name__ == "__main__":
    mcp.run(transport="sse", host="0.0.0.0", port=8765)
```

### Важные детали tools/

Все инструменты, которым нужен shell, должны использовать `subprocess.run` с
`capture_output=True, text=True` — **никакого** paramiko внутри MCP-сервера,
потому что сервер выполняется прямо на целевой машине.

Инструменты GitLab и YouTrack используют `httpx` (async-ready HTTP-клиент)
и читают токены из переменных окружения `GITLAB_TOKEN` и `YOUTRACK_TOKEN`.

Для инструментов kubectl путь к kubeconfig нужно явно задать:
`KUBECONFIG=/etc/rancher/k3s/k3s.yaml` — добавить в окружение контейнера.

### Dockerfile

```dockerfile
FROM python:3.12-slim

WORKDIR /app

# kubectl для K3s-операций
RUN apt-get update && apt-get install -y curl && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl && apt-get clean

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8765
CMD ["python", "server.py"]
```

### docker-compose.mcp.yml

```yaml
version: "3.8"

services:
  homelab-mcp:
    build: /opt/homelab-mcp
    image: homelab-mcp:latest
    container_name: homelab-mcp
    restart: unless-stopped
    ports:
      - "8765:8765"
    volumes:
      # Доступ к Docker socket для управления контейнерами
      - /var/run/docker.sock:/var/run/docker.sock
      # kubeconfig для kubectl
      - /etc/rancher/k3s/k3s.yaml:/etc/rancher/k3s/k3s.yaml:ro
    environment:
      - GITLAB_TOKEN=${GITLAB_TOKEN}
      - YOUTRACK_TOKEN=${YOUTRACK_TOKEN}
      - KUBECONFIG=/etc/rancher/k3s/k3s.yaml
      - GITLAB_URL=http://192.168.1.74:8080
      - YOUTRACK_URL=http://192.168.1.74:8088
    network_mode: host   # нужно для health-check'ов внутренних сервисов
```

### requirements.txt

```
fastmcp>=0.1.0
httpx>=0.27.0
docker>=7.0.0
```

### .env файл на сервере

Перед деплоем создать `/opt/homelab-mcp/.env` с содержимым:

```
GITLAB_TOKEN=<значение из ~/.gitlab-token>
YOUTRACK_TOKEN=<значение из ~/.bashrc>
```

## Деплой (пошагово для агента)

Деплой выполняется через SSH с помощью Python + paramiko (password-based auth,
как описано в CLAUDE.md). Последовательность шагов:

1. Создать директорию `/opt/homelab-mcp/` на сервере.
2. Загрузить все файлы проекта через paramiko SFTP (write каждый файл).
3. Создать `.env` файл, прочитав токены из `~/.gitlab-token` и `~/.bashrc`.
4. Запустить сборку: `docker compose -f /opt/homelab-mcp/docker-compose.mcp.yml build`.
5. Запустить контейнер: `docker compose -f /opt/homelab-mcp/docker-compose.mcp.yml up -d`.
6. Проверить, что сервер отвечает: `curl http://localhost:8765/` должен вернуть ответ.

## Подключение агента к MCP-серверу

После деплоя добавить в `E:\LocOll\.claude\settings.json` (и в `E:\EL\errorlens\.claude\settings.json`):

```json
{
  "mcpServers": {
    "homelab": {
      "type": "sse",
      "url": "http://192.168.1.74:8765/sse"
    }
  }
}
```

Для удалённого доступа (Tailscale):

```json
{
  "mcpServers": {
    "homelab": {
      "type": "sse",
      "url": "http://100.81.243.12:8765/sse"
    }
  }
}
```

После этого агент видит все инструменты MCP-сервера напрямую, без SSH-кода в задачах.

## Критерии готовности

- MCP-сервер запущен как Docker-контейнер и работает при рестарте сервера (restart: unless-stopped)
- `curl http://192.168.1.74:8765/sse` отвечает без ошибок
- Claude Code видит все зарегистрированные инструменты (проверить через `/mcp` в агенте)
- Проверена работа минимум трёх инструментов: `get_services_status`, `get_docker_ps`, `run_health_check`
- settings.json обновлены в обоих проектах (LocOll и ErrorLens)

## Приоритет

Высокий. Этот сервер — фундаментальное улучшение архитектуры агентной работы
для всех текущих и будущих проектов на homelab.

---

## ✅ Статус: ВЫПОЛНЕНА

**Дата завершения:** 2026-03-11

**Что сделано:**
- Создан MCP-сервер на Python (fastmcp 3.1.0) с 9 инструментами
- Задеплоен как Docker-контейнер на сервере (`/opt/homelab-mcp/`, порт 8765, SSE transport)
- Docker CLI установлен внутри контейнера для доступа к Docker socket
- `.mcp.json` добавлен в оба проекта (LocOll и ErrorLens)
- Проверена работа: `get_system_metrics`, `get_services_status`, `run_health_check` — работают
- `get_docker_ps`, `get_service_logs` — исправлены (добавлен docker-ce-cli в образ)
- Контейнер с `restart: unless-stopped` для автозапуска

**Отклонения от плана:**
- Убраны kubernetes tools (K3s удалён с сервера)
- Убраны gitlab/youtrack tools (нет токенов, нет API)
- MCP подключён через `.mcp.json` вместо `settings.json` (правильный формат Claude Code)

**Обнаруженные проблемы (перешли в следующие задачи):**
- Проверка инструментов через `/mcp` в агенте требует перезапуск сессии Claude Code
- `get_service_logs` для не-Docker сервисов (Ollama) не поддерживается — только Docker контейнеры
