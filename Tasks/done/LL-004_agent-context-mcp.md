# LL-004: agent-context — локальный MCP-сервер управления контекстом проектов

> Статус: Готово к разработке  
> Дата создания: 2026-03-13

---

## Цель

Создать локальный MCP-сервер который решает одну проблему: **потерю контекста при переключении между проектами и между сессиями**. Сервер становится единственным источником правды о каждом проекте — изолированным, структурированным, постоянным.

Сервер обслуживает два клиента одновременно через stdio:

- **Claude Code** — запускается из директории проекта, проект определяется автоматически по `process.cwd()`
- **Claude Desktop** — запускается из системной директории, проект передаётся явно через параметр `project_path` при вызове `start_session`

Оба клиента работают с одной и той же БД. Никакого HTTP, никаких портов — только stdio и SQLite.

---

## Техническая среда

Операционная система: **Windows**, рабочая директория сервера: `C:\Users\<user>\.agent-context\`.

Транспорт MCP: **stdio** — сервер запускается как дочерний процесс клиента.

База данных: **SQLite**, файл `~/.agent-context/context.db`.

Рантайм: **Node.js 18+**.

SDK: `@modelcontextprotocol/sdk` (официальный).

---

## Файловая структура

```
~/.agent-context/
├── server.js            ← точка входа
├── package.json
├── db.js                ← инициализация схемы и все SQL-запросы
├── project-resolver.js  ← логика определения активного проекта
├── tools/
│   ├── start_session.js
│   ├── end_session.js
│   ├── checkpoint.js
│   ├── get_context.js
│   ├── update_conventions.js
│   └── list_projects.js
├── registry.json        ← реестр проектов, редактируется вручную
└── context.db           ← создаётся автоматически
```

---

## Конфигурация клиентов

### Claude Desktop — `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "agent-context": {
      "command": "node",
      "args": ["C:/Users/<user>/.agent-context/server.js"]
    }
  }
}
```

### Claude Code — `.claude/claude.json` в директории проекта

```json
{
  "mcpServers": {
    "agent-context": {
      "command": "node",
      "args": ["C:/Users/<user>/.agent-context/server.js"]
    }
  }
}
```

---

## Схема базы данных

База создаётся автоматически при первом запуске.

```sql
CREATE TABLE IF NOT EXISTS project_conventions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project     TEXT NOT NULL,
    layer       TEXT NOT NULL CHECK(layer IN ('static', 'global')),
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project, key)
);

CREATE TABLE IF NOT EXISTS sessions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    project       TEXT NOT NULL,
    started_at    TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at      TEXT,
    summary       TEXT,
    pending_tasks TEXT,   -- JSON-массив строк
    open_files    TEXT,   -- JSON-массив строк
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK(status IN ('active', 'completed'))
);

CREATE TABLE IF NOT EXISTS checkpoints (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  INTEGER NOT NULL REFERENCES sessions(id),
    note        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

---

## Реестр проектов (registry.json)

Файл создаётся агентом при инициализации. Разработчик редактирует вручную.

```json
{
  "projects": {
    "E:/LocOll": {
      "name": "LocOll",
      "type": "experiment",
      "description": "Экспериментальный homelab-портал: Go + htmx + DuckDB WASM"
    },
    "E:/HomeLab": {
      "name": "HomeLab",
      "type": "infra",
      "description": "Управление домашней лабораторией"
    },
    "E:/WarehouseHub": {
      "name": "WarehouseHub",
      "type": "backend",
      "youtrack_prefix": "WH",
      "description": "Production warehouse backend, K3s"
    },
    "E:/EL": {
      "name": "ErrorLens",
      "type": "backend",
      "teammates": ["Ирек", "Кирилл"],
      "description": "Командный проект с Docker-инфраструктурой"
    }
  },
  "global": {
    "server_host": "192.168.1.74",
    "server_user": "flomaster",
    "server_ram": "24GB",
    "server_tailscale": "100.81.243.12",
    "k3s_prod_namespace": "warehouse",
    "k3s_dev_namespace": "warehouse-dev",
    "gitlab_api_port": "8080",
    "youtrack_port": "8088",
    "notes": "Локальная сборка Docker запрещена — OOM. Только GitLab CI/CD."
  }
}
```

---

## Определение активного проекта (project-resolver.js)

Это критически важный модуль. Логика **Вариант C**: при вызове `start_session` проект определяется один раз и фиксируется в памяти процесса. Все последующие tool-вызовы берут проект из этого кеша.

### Порядок определения при `start_session`

1. Если передан параметр `project_path` — использовать его. Это сценарий Claude Desktop.
2. Иначе взять `process.cwd()`. Это сценарий Claude Code.
3. Нормализовать путь: привести к нижнему регистру, заменить все `\` на `/`, убрать trailing-слэш.
4. Сравнить с ключами из `registry.json` (те же нормализации). Совпадение — точное или `cwd` является дочерней директорией зарегистрированного пути.
5. Если найдено — сохранить в модульной переменной `let activeProject = null`.
6. Если не найдено — вернуть ошибку: `"❌ Ошибка: директория '<path>' не зарегистрирована. Добавьте проект в registry.json"`.

### Поведение остальных tools

Все tools кроме `start_session` и `list_projects` берут активный проект из кеша. Если кеш пуст — возвращать: `"❌ Ошибка: сессия не инициализирована. Вызовите start_session."`.

Изоляция гарантируется на уровне SQL: каждый SELECT включает `WHERE project = ?`.

---

## Инициализация (seed)

При каждом старте сервера: читать `registry.json` и для каждого проекта выполнить `INSERT OR IGNORE` в `project_conventions`:

- `key = 'description'`, `value` из поля `description`
- `key = 'type'`, `value` из поля `type`

Для секции `global` — те же вставки с `project = '__global__'`, `layer = 'global'`, по одной записи на каждый ключ. Новые проекты добавленные в `registry.json` подхватываются при следующем старте автоматически.

---

## Обработка зависших сессий

При каждом вызове `start_session` — до создания новой сессии — проверить: есть ли для этого проекта сессия со статусом `active`, у которой `started_at` старше **24 часов**. Если есть — автоматически закрыть: `status = 'completed'`, `summary = 'Сессия закрыта автоматически (timeout 24h)'`, `ended_at = datetime('now')`. Уведомить агента в ответе строкой `⚠️ Предыдущая сессия закрыта по таймауту`.

---

## MCP Tools

### `start_session`

**Параметры:** `project_path` (string, необязательный) — явное указание проекта, используется в Claude Desktop.

**Логика:** определить проект → закрыть зависшие сессии → загрузить `static` и `global` conventions → найти последнюю завершённую сессию → создать новую запись `active` → вернуть контекст.

**Формат ответа:**

```
=== Контекст проекта: LocOll ===

[Глобальное окружение]
server: 192.168.1.74 (user: flomaster, 24GB RAM)
k3s: prod → namespace warehouse, dev → warehouse-dev
gitlab_api_port: 8080 | youtrack_port: 8088
notes: Локальная сборка Docker запрещена — OOM. Только GitLab CI/CD.

[Соглашения проекта]
type: experiment
description: Экспериментальный homelab-портал: Go + htmx + DuckDB WASM

[Последняя сессия: 2 дня назад]
Итог: DuckDB WASM fix завершён
Не завершено: LL-004
Файлы: main.go, handlers.go

[Новая сессия открыта: ID 5]
```

Если `pending_tasks` или `open_files` пустые — строки не выводить. Если предыдущих сессий нет — секцию `[Последняя сессия]` не выводить.

---

### `end_session`

**Параметры:** `summary` (string, обязательный), `pending_tasks` (string[], необязательный), `open_files` (string[], необязательный).

**Логика:** найти активную сессию. Если нет — ошибка. Обновить поля, `status = 'completed'`, записать `ended_at`. `pending_tasks` и `open_files` сериализовать через `JSON.stringify`. Очистить кеш активного проекта.

**Ответ:** `"✅ Сессия завершена. Сохранено: <summary>"`

---

### `checkpoint`

**Параметры:** `note` (string, обязательный).

**Логика:** найти активную сессию. Если нет — ошибка. Добавить запись в `checkpoints`.

**Ответ:** `"📍 Чекпоинт сохранён: <note>"`

---

### `get_context`

**Параметры:** нет.

**Логика:** аналогично `start_session`, но без создания новой записи в `sessions`. Если есть активная сессия — дополнительно выводить чекпоинты текущей сессии.

**Дополнительная секция в ответе:**

```
[Чекпоинты текущей сессии]
14:23 — Завершил миграцию схемы
15:47 — Подключил Redis, тесты зелёные
```

---

### `update_conventions`

**Параметры:** `key` (string, обязательный), `value` (string, обязательный), `layer` (string, необязательный, по умолчанию `static`).

**Логика:** UPSERT в `project_conventions`. Если `layer = 'global'` — `project = '__global__'`.

**Ответ:** `"✅ Соглашение '<key>' сохранено для проекта <name>"`

---

### `list_projects`

**Параметры:** нет. Не требует активной сессии.

**Логика:** прочитать `registry.json`. Для каждого проекта найти дату последней сессии. Если есть активная сессия — маркер `● активна`.

**Формат ответа:**

```
Зарегистрированные проекты:

LocOll       [experiment] Последняя сессия: сегодня        ● активна
HomeLab      [infra]      Последняя сессия: 3 дня назад
WarehouseHub [backend]    Последняя сессия: 5 дней назад
ErrorLens    [backend]    Сессий нет
```

---

## Обработка ошибок

Все ошибки — текстовые сообщения агенту, формат `"❌ Ошибка: <описание>"`. Обрабатывать явно: проект не найден, кеш пуст, нет активной сессии, `registry.json` не найден или невалидный JSON, ошибка SQLite. При чтении `pending_tasks` / `open_files` — `JSON.parse` в try/catch, при ошибке возвращать `[]`.

---

## Технические требования

```json
{
  "name": "agent-context",
  "version": "1.0.0",
  "type": "module",
  "dependencies": {
    "@modelcontextprotocol/sdk": "latest",
    "better-sqlite3": "latest"
  }
}
```

`better-sqlite3` — синхронный API, критично для stdio-транспорта. Асинхронный `sqlite3` не использовать.

При успешном старте выводить в **stderr**:

```
[agent-context] MCP сервер запущен
[agent-context] Загружено проектов: 4
[agent-context] БД: C:/Users/<user>/.agent-context/context.db
```

---

## Порядок выполнения

```
1. Создать директорию ~/.agent-context на Windows-машине
2. Написать package.json, npm install
3. Написать db.js — схема + все SQL-функции
4. Написать project-resolver.js — логика определения и кеш
5. Написать tools/ — по одному файлу на каждый tool
6. Написать server.js — точка входа, регистрация tools, stdio транспорт
7. Создать registry.json с проектами (пути из файловой структуры ноута)
8. Запустить node server.js, проверить старт и список tools
9. Проверить каждый критерий готовности
```

---

## Критерии готовности

| Проверка | Как убедиться |
|---|---|
| Сервер стартует | `npm install && node server.js` без ошибок, stderr показывает 3 строки |
| Tools видны | Claude Code / Claude Desktop показывает 6 инструментов |
| `start_session` в зарегистрированной директории | Возвращает корректный контекст |
| `start_session` с `project_path` | Работает из любой директории (сценарий Desktop) |
| `start_session` в незарегистрированной директории | Понятная ошибка |
| Изоляция данных | Данные проекта A недоступны при активном проекте B |
| Восстановление состояния | После `end_session` + повторный `start_session` → предыдущее состояние |
| `update_conventions` персистентна | Сохранённое соглашение появляется в следующем `get_context` |
| `list_projects` полная | Все проекты из registry.json + дата + маркер активной |
| Зависшая сессия закрывается | Сессия старше 24h автоматически завершается |

---

## Запрещено

- Использовать асинхронный `sqlite3` вместо `better-sqlite3`
- Делать SELECT без `WHERE project = ?` в запросах данных проекта
- Выводить что-либо в stdout кроме MCP-протокола (логи — только в stderr)
- Хранить `pending_tasks` и `open_files` как plain text — только JSON
- Обращаться к данным другого проекта из любого tool кроме `list_projects`

---

## ✅ Статус: ВЫПОЛНЕНА

**Дата завершения:** 2026-03-13

**Что сделано:**
- Создана директория `~/.agent-context/` со всей файловой структурой (server.js, db.js, project-resolver.js, tools/, registry.json)
- Реализована SQLite-схема: project_conventions, sessions, checkpoints
- Реализованы 6 MCP-инструментов: start_session, end_session, checkpoint, get_context, update_conventions, list_projects
- Автоопределение проекта по cwd (Claude Code) и по project_path (Claude Desktop)
- Изоляция данных между проектами через WHERE project = ?
- Автозакрытие зависших сессий старше 24ч
- Seed из registry.json при каждом старте сервера
- Настроен MCP в `.mcp.json` (LocOll) и `claude_desktop_config.json` (Claude Desktop)

**Отклонения от плана:**
- inputSchema реализован через Zod-схемы вместо plain JSON schema — требование SDK @modelcontextprotocol/sdk
- Регистрация tools через `server.tool()` с Zod raw shapes вместо `registerTool()` с JSON schema

**Обнаруженные проблемы:**
- Нет
