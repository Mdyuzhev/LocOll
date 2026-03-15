# LL-008: claude-bridge — HTTP-эндпоинт для прямого вызова MCP-инструментов из браузера

> Статус: Готово к разработке
> Дата создания: 2026-03-15

---

## Цель

Добавить в homelab-mcp эндпоинт `/claude-bridge` который позволяет вызывать
MCP-инструменты через обычный HTTP POST без протокольного церемониала MCP
(initialize → session_id → tools/call). Это открывает возможность Claude в
claude.ai работать с сервером напрямую через Claude in Chrome — без Claude Desktop
и без Claude Code.

Сейчас для одного вызова инструмента нужно: открыть вкладку на 8765, сделать
`initialize`, вытащить `mcp-session-id` из заголовка, и только потом вызвать
инструмент. Каждый новый блок кода — новая сессия. Это работает, но хрупко и
многословно. После этой задачи — один fetch, один ответ.

---

## Техническая среда

**Сервер**: homelab-mcp, `/opt/homelab-mcp/server.py`, Docker-контейнер
`homelab-mcp`, порт 8765, `network_mode: host`.

**FastMCP версия**: 3.1.0 — нативно поддерживает `@mcp.custom_route(path, methods)`
для регистрации произвольных HTTP-маршрутов. Обработчик — async-функция принимающая
`starlette.requests.Request` и возвращающая `starlette.responses.Response`.

**Деплой**: пересборка через `docker compose -f /opt/homelab-mcp/docker-compose.mcp.yml
up -d --build` (CI/CD не настроен для этого проекта — делается напрямую).

---

## Что добавляется

### Новый файл `tools/bridge.py`

Вся логика bridge живёт отдельно от `server.py` — чистое разделение.

```python
"""claude-bridge — упрощённый HTTP-эндпоинт для вызова MCP-инструментов.

Позволяет Claude в claude.ai вызывать инструменты через один fetch-запрос
без инициализации MCP-сессии.
"""
from starlette.requests import Request
from starlette.responses import JSONResponse


# Реестр разрешённых инструментов — whitelist безопасности.
# Добавлять только read-only и безопасные write-операции.
ALLOWED_TOOLS = {
    "get_system_metrics",
    "get_services_status",
    "get_docker_ps",
    "get_server_info",
    "run_health_check",
    "get_service_logs",
    "grep_docker_logs",
    "run_shell_command",
    "exec_in_container",
    "list_notes",
    "add_note",
    "complete_note",
    "restart_service",
    "start_service",
    "stop_service",
    "compose_up",
}


async def claude_bridge_handler(request: Request) -> JSONResponse:
    """Обработчик /claude-bridge.

    Принимает: POST {"tool": "имя_инструмента", "args": {...}}
    Возвращает: {"ok": true, "result": ...} или {"ok": false, "error": "..."}
    """
    # Разбираем тело запроса
    try:
        body = await request.json()
    except Exception:
        return JSONResponse({"ok": False, "error": "Invalid JSON body"}, status_code=400)

    tool_name = body.get("tool")
    args = body.get("args", {})

    # Проверяем что инструмент указан
    if not tool_name:
        return JSONResponse({"ok": False, "error": "Field 'tool' is required"}, status_code=400)

    # Whitelist — защита от случайного вызова деструктивных операций
    if tool_name not in ALLOWED_TOOLS:
        return JSONResponse(
            {"ok": False, "error": f"Tool '{tool_name}' is not allowed via bridge"},
            status_code=403,
        )

    # Вызываем инструмент через реестр FastMCP
    # request.app — это Starlette-приложение FastMCP, из него достаём сервер
    mcp_server = request.app.state.mcp_server
    try:
        result = await mcp_server.call_tool(tool_name, args)
        # result — объект FastMCP, из него достаём текстовое содержимое
        content = result.content[0].text if result.content else ""
        # Если это JSON — парсим, иначе возвращаем как строку
        import json
        try:
            parsed = json.loads(content)
        except Exception:
            parsed = content
        return JSONResponse({"ok": True, "result": parsed})
    except Exception as e:
        return JSONResponse({"ok": False, "error": str(e)}, status_code=500)
```

### Изменения в `server.py`

После создания объекта `mcp` и регистрации всех инструментов — добавить три вещи:

**1. Импорт и CORS middleware:**
```python
from starlette.middleware.cors import CORSMiddleware
from tools.bridge import claude_bridge_handler
```

**2. Регистрация маршрута через декоратор:**
```python
@mcp.custom_route("/claude-bridge", methods=["POST"])
async def claude_bridge(request: Request) -> Response:
    return await claude_bridge_handler(request)

@mcp.custom_route("/claude-bridge", methods=["OPTIONS"])
async def claude_bridge_options(request: Request) -> Response:
    """CORS preflight для браузерных запросов с других доменов."""
    from starlette.responses import Response
    r = Response()
    r.headers["Access-Control-Allow-Origin"] = "*"
    r.headers["Access-Control-Allow-Methods"] = "POST, OPTIONS"
    r.headers["Access-Control-Allow-Headers"] = "Content-Type"
    return r
```

**3. Передача ссылки на mcp в app.state при запуске:**

FastMCP запускает Starlette-приложение — нам нужно положить ссылку на `mcp` в
`app.state` чтобы `claude_bridge_handler` мог достать его через `request.app.state`.
Делается через lifespan или startup-хук после `mcp.run()`. Точный способ зависит
от версии FastMCP — агент должен проверить актуальный API в документации или
исходниках: `docker exec homelab-mcp python3 -c "import fastmcp; help(fastmcp.FastMCP.run)"`.

Альтернативный подход если `app.state` недоступен — импортировать функции
инструментов напрямую в `bridge.py` и вызывать их как обычные Python-функции,
минуя FastMCP целиком:

```python
# Прямой вызов без FastMCP — проще и надёжнее
from tools.services import get_system_metrics, get_services_status, get_docker_ps
from tools.shell import run_shell_command
# и т.д.

TOOL_REGISTRY = {
    "get_system_metrics": get_system_metrics,
    "get_services_status": get_services_status,
    "get_docker_ps": get_docker_ps,
    "run_shell_command": run_shell_command,
    # ... все остальные
}

# В обработчике:
fn = TOOL_REGISTRY.get(tool_name)
result = fn(**args)  # или await fn(**args) если async
```

Этот подход проще, не зависит от внутреннего API FastMCP и гарантированно
работает — агент выбирает его если интроспекция `app.state` окажется сложной.

### Новый файл `tools/mcp_client.js`

Статический JS-хелпер для браузера — доступен по `/static/mcp-client.js`.
Claude загружает его один раз и дальше работает через чистый API.

```javascript
/**
 * Минимальный клиент для /claude-bridge
 * Использование:
 *   const mcp = new MCPBridgeClient('http://100.81.243.12:8765');
 *   const metrics = await mcp.call('get_system_metrics');
 *   const containers = await mcp.call('get_docker_ps');
 *   const result = await mcp.call('run_shell_command', {command: 'docker ps'});
 */
class MCPBridgeClient {
  constructor(baseUrl) {
    this.url = baseUrl.replace(/\/$/, '') + '/claude-bridge';
  }

  async call(tool, args = {}) {
    const resp = await fetch(this.url, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({tool, args})
    });
    const data = await resp.json();
    if (!data.ok) throw new Error(data.error);
    return data.result;
  }
}

// Автоэкспорт для использования в браузерных скриптах
if (typeof window !== 'undefined') window.MCPBridgeClient = MCPBridgeClient;
```

Этот файл нужно положить в `/opt/homelab-mcp/static/` и зарегистрировать как
статический маршрут в `server.py`:

```python
@mcp.custom_route("/static/mcp-client.js", methods=["GET"])
async def serve_mcp_client(request: Request) -> Response:
    from starlette.responses import FileResponse
    return FileResponse("/app/static/mcp-client.js",
                        media_type="application/javascript")
```

---

## Порядок выполнения

Сначала убедиться что контейнер запущен и `/claude-bridge` ещё не существует:

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8765/claude-bridge
# Ожидаем 404 — эндпоинта нет
```

Затем реализовать в таком порядке:

1. Создать `tools/bridge.py` с прямым вызовом функций через `TOOL_REGISTRY`
2. Добавить регистрацию маршрута и CORS OPTIONS в `server.py`
3. Создать `static/mcp-client.js`
4. Пересобрать контейнер: `docker compose -f /opt/homelab-mcp/docker-compose.mcp.yml up -d --build`
5. Проверить критерии готовности

---

## Критерии готовности

| Проверка | Команда |
|---|---|
| Эндпоинт отвечает | `curl -s -X POST http://localhost:8765/claude-bridge -H "Content-Type: application/json" -d '{"tool":"get_system_metrics","args":{}}' \| python3 -m json.tool` |
| Возвращает метрики | В ответе `ok: true` и поля `cpu_percent`, `ram_used_mb` |
| Whitelist работает | `curl ... -d '{"tool":"manage_bot","args":{"action":"stop"}}` → `ok: false, error: not allowed` |
| Неизвестный инструмент | `{"tool":"nonexistent"}` → `ok: false` с понятным сообщением |
| CORS заголовки | `curl -I -X OPTIONS ... -H "Origin: http://example.com"` → `Access-Control-Allow-Origin: *` |
| JS-хелпер доступен | `curl -s http://localhost:8765/static/mcp-client.js \| head -5` — видим класс MCPBridgeClient |
| run_shell_command работает | `{"tool":"run_shell_command","args":{"command":"hostname"}}` → имя хоста в result |
| list_notes работает | `{"tool":"list_notes","args":{}}` → массив заметок (или пустой список) |

---

## Запрещено

- Добавлять деструктивные инструменты в ALLOWED_TOOLS без явного обсуждения
- Убирать CORS OPTIONS-хендлер — без него браузер будет блокировать запросы
- Хардкодить адрес сервера в `mcp-client.js` — он принимается в конструкторе
- Трогать существующий MCP-протокол — `/claude-bridge` это дополнительный маршрут, не замена

---

## ✅ Статус: ВЫПОЛНЕНА

**Дата завершения:** 2026-03-15

**Что сделано:**
- Создан `tools/bridge.py` с TOOL_REGISTRY (16 инструментов) и прямым вызовом функций (без интроспекции FastMCP app.state)
- Обновлён `server.py`: маршруты POST /claude-bridge, OPTIONS /claude-bridge (CORS), GET /claude-bridge/tools, GET /static/mcp-client.js
- Создан `static/mcp-client.js` — JS-хелпер класс MCPBridgeClient для браузера
- Контейнер пересобран и запущен
- Все 8 критериев готовности пройдены

**Отклонения от плана:**
- Выбран альтернативный подход (прямой вызов функций через TOOL_REGISTRY) вместо app.state — проще и надёжнее
- Добавлен бонусный эндпоинт GET /claude-bridge/tools для интроспекции доступных инструментов
- Sync-функции оборачиваются через asyncio.to_thread для неблокирующего вызова
