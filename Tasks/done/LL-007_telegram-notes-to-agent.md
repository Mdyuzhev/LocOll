# LL-007: telegram-notes-to-agent — заметки из Telegram в контекст агента

> Статус: Готово к разработке
> Дата создания: 2026-03-13

---

## Цель

Ты пишешь `/note купить молоко` в Telegram — заметка сохраняется на сервере.
Когда открываешь Claude Desktop и вызываешь `/init`, агент видит непрочитанные
заметки в своей сводке и напоминает тебе о них. Отметил выполненной — `/done 3`.

Никакого лишнего синтаксиса, никаких промежуточных шагов. Мобильный ввод →
контекст агента.

---

## Техническая среда

Три компонента которые уже существуют и не требуют новой инфраструктуры:

**homelab-mcp** (`~/projects/homelab-mcp/` на сервере, порт 8765) — здесь добавляем
хранилище заметок (SQLite) и два новых MCP-инструмента.

**homelab-bot** (`~/projects/homelab-bot/` на сервере, Docker) — здесь добавляем
три Telegram-команды: `/note`, `/notes`, `/done`.

**agent-context** (`~/.agent-context/` на Windows) — здесь обновляем
`server-bridge.js` чтобы заметки появлялись в сводке `start_session`.

homelab-mcp уже работает с `network_mode: host` и volume-монтированием
`/home/flomaster`. homelab-bot тоже `network_mode: host` — то есть оба контейнера
видят один и тот же `localhost:8765`. Новой сети не нужно.

---

## Часть 1 — homelab-mcp: хранилище заметок

### Новый файл `tools/notes.py`

SQLite-база для заметок. Файл базы — `/home/flomaster/.homelab-notes.db` (уже в
смонтированном volume, переживёт пересборку контейнера).

```python
# Схема таблицы
CREATE TABLE IF NOT EXISTS notes (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    text      TEXT    NOT NULL,
    project   TEXT,           -- опционально: 'locoll', 'moex' и т.д.
    done      INTEGER DEFAULT 0,
    created   INTEGER NOT NULL,  -- unix timestamp
    done_at   INTEGER            -- unix timestamp, NULL пока не выполнена
)
```

Три инструмента:

**`add_note(text: str, project: str | None = None) -> dict`**
Создаёт заметку. Возвращает `{"id": 42, "text": "...", "created": "..."}`.

**`list_notes(project: str | None = None, include_done: bool = False) -> list[dict]`**
Возвращает заметки. По умолчанию только невыполненные (`done=0`), отсортированные
по дате создания (новые последними). Каждая запись: `{id, text, project, done,
created_human}`, где `created_human` — строка вида `"сегодня 14:32"` или
`"вчера 09:15"` или `"11 марта 10:00"` (московское время, TZ=Europe/Moscow).

**`complete_note(note_id: int) -> dict`**
Помечает заметку выполненной (`done=1`, `done_at=now`). Возвращает
`{"ok": true, "id": 42}` или `{"ok": false, "error": "not found"}`.

Регистрируем в `server.py`:
```python
from tools.notes import add_note, list_notes, complete_note
mcp.tool()(add_note)
mcp.tool()(list_notes)
mcp.tool()(complete_note)
```

---

## Часть 2 — homelab-bot: Telegram-команды

Добавляем новый файл `bot/commands/notes.py` с тремя хендлерами.
HTTP-клиент к homelab-mcp уже есть в `bot/clients/locoll.py` — по аналогии
создаём `bot/clients/mcp_notes.py` или добавляем методы в существующий клиент.

### `/note <текст>` — добавить заметку

```
/note купить кофе
/note locoll: проверить SSE-брокер на утечки
```

Опциональный префикс `проект:` перед текстом — если указан, заполняет поле
`project`. Разбирается простым regex: `^(\w+):\s+(.+)$`. Если не совпало —
`project=None`.

Бот вызывает `add_note` в homelab-mcp через HTTP и отвечает:
```
✅ Заметка #42 сохранена
купить кофе
```

### `/notes` — список невыполненных заметок

Вызывает `list_notes(include_done=False)`. Если заметок нет:
```
📝 Нет активных заметок
```

Если есть:
```
📝 Заметки (3)

#41  вчера 09:15  — проверить SSE-брокер на утечки  [locoll]
#42  сегодня 14:32  — купить кофе
#43  сегодня 15:01  — написать тест для watcher

Отметить выполненной: /done <id>
```

### `/done <id>` — отметить выполненной

```
/done 42
```

Вызывает `complete_note(42)`. Отвечает:
```
✅ Заметка #42 выполнена
```

Если id не найден или уже выполнена:
```
❌ Заметка #42 не найдена или уже выполнена
```

### Регистрация хендлеров в `bot/main.py`

По аналогии с существующими командами:
```python
from bot.commands.notes import note_command, notes_command, done_command

application.add_handler(CommandHandler("note", note_command))
application.add_handler(CommandHandler("notes", notes_command))
application.add_handler(CommandHandler("done", done_command))
```

Также добавить команды в `set_my_commands` чтобы они появились в меню бота:
```
/note <текст> — добавить заметку
/notes — список заметок
/done <id> — отметить выполненной
```

---

## Часть 3 — agent-context: заметки в сводке start_session

Файл `~/.agent-context/server-bridge.js` уже умеет ходить к homelab-mcp. Нужно
добавить запрос `list_notes` и вставить секцию в вывод `start_session` и
`get_context`.

### Логика

Добавить в `fetchServerDigest` (или рядом) функцию `fetchPendingNotes()`:

```js
async function fetchPendingNotes() {
  // HTTP POST к homelab-mcp: вызов инструмента list_notes
  // include_done: false, project: null (все заметки без фильтра)
  // timeout: 3 секунды, при ошибке — вернуть null
}
```

Формат секции в ответе `start_session` / `get_context`:

Если заметок нет — секцию не добавлять вообще (не засорять сводку).

Если есть непрочитанные заметки:
```
[📝 Заметки (3)]
#41  вчера 09:15  — проверить SSE-брокер на утечки  [locoll]
#42  сегодня 14:32  — купить кофе
#43  сегодня 15:01  — написать тест для watcher

Отметить: complete_note(id) через mcp__homelab__complete_note
```

Секция добавляется **после** серверного дайджеста событий, перед закрывающей
строкой ответа.

### Как вызвать инструмент через HTTP

homelab-mcp использует FastMCP со `streamable-http` транспортом. Вызов
инструмента — POST на `/mcp` с телом MCP-протокола. Нужно посмотреть как это
уже делается в существующем `server-bridge.js` для `fetchServerDigest` и
использовать точно такой же паттерн для `list_notes`.

---

## Порядок выполнения

Сначала проверить что homelab-mcp доступен: `curl -s localhost:8765` через
paramiko. Посмотреть реальный формат MCP-запросов в существующем `server-bridge.js`
— писать клиентов под реальность.

Затем по частям:

1. `tools/notes.py` в homelab-mcp → пересобрать контейнер → проверить инструменты
   через прямой HTTP-вызов.
2. `bot/commands/notes.py` + `bot/clients/mcp_notes.py` в homelab-bot → перезапустить
   контейнер → проверить `/note`, `/notes`, `/done` в Telegram.
3. Обновить `server-bridge.js` в agent-context → перезапустить сервер
   (`node ~/.agent-context/server.js`) → проверить что заметки появляются в выводе
   `start_session`.

**Пересборка homelab-mcp** — через paramiko:
```bash
cd ~/projects/homelab-mcp && docker compose -f docker-compose.mcp.yml up -d --build
```

**Пересборка homelab-bot** — через paramiko:
```bash
cd ~/projects/homelab-bot && docker compose up -d --build
```

---

## Критерии готовности

| Проверка | Как убедиться |
|---|---|
| `/note купить кофе` | Бот отвечает `✅ Заметка #N сохранена` |
| `/note locoll: проверить watcher` | Сохраняется с `project='locoll'` |
| `/notes` | Показывает список с id, временем и текстом |
| `/notes` пустой | Отвечает `📝 Нет активных заметок` |
| `/done <id>` | Заметка помечается, исчезает из `/notes` |
| `/done 9999` | Отвечает `❌ Заметка #9999 не найдена` |
| `start_session` с заметками | Секция `[📝 Заметки]` появляется в сводке |
| `start_session` без заметок | Секция отсутствует, сводка не засорена |
| Graceful degradation | При недоступном homelab-mcp бот отвечает ошибкой, `start_session` продолжает работу без секции |
| Перезапуск контейнеров | Заметки переживают `docker compose restart` (SQLite в смонтированном volume) |

---

## Запрещено

- Хранить заметки в памяти процесса — только SQLite в volume
- Использовать отдельный порт или новый сервис — только существующий homelab-mcp
- Голый `ssh`, `sshpass` — только paramiko
- Тянуть новые модели Ollama
