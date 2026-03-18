# Команда /onboard <ProjectName> — взять проект под контроль LocOll

Аргумент: $1 = имя проекта (например "Sharmanka")

Сразу написать: **Подключаю проект $1...**

**Шаг 1: Найти проект**
Прочитать `.git/config` в `E:\$1\` чтобы получить git remote URL.
Вызвать `identify_project("E:/$1", git_remote)` для классификации.

**Шаг 2: Зарегистрировать в agent-context**
Вызвать `register_project`:
  path = "E:/$1"
  name = "$1"
  type = (из identify_project)
  description = (из git remote + имени)

**Шаг 3: Создать .mcp.json**
Создать файл `E:\$1\.mcp.json`:
```json
{
  "mcpServers": {
    "homelab": {
      "type": "http",
      "url": "http://192.168.1.74:8765/mcp"
    },
    "agent-context": {
      "type": "http",
      "url": "http://192.168.1.74:8766/mcp"
    }
  }
}
```

**Шаг 4: Создать .claude/settings.json**
Создать `E:\$1\.claude\settings.json`:
```json
{
  "permissions": {
    "allow": [
      "Bash(*)", "Read(*)", "Write(*)", "Edit(*)",
      "Glob(*)", "Grep(*)", "WebFetch(*)", "WebSearch(*)",
      "mcp__homelab__*",
      "mcp__agent-context__*"
    ]
  }
}
```

**Шаг 5: Создать базовый CLAUDE.md**
Создать `E:\$1\.claude\CLAUDE.md` по шаблону:

```markdown
# CLAUDE.md — $1

## Начало работы
При открытии нового чата — запустить `/init`.

## Чекпоинты — обязательно
1. При завершении каждого шага — mcp__agent-context__checkpoint
2. Каждые 5 вызовов инструментов — checkpoint
3. Перед завершением задачи — end_session с итогом

## О проекте
$1 — [TODO: описание проекта]
GitHub: [TODO: ссылка на репо]

## Деплой
[TODO: заполнить после изучения проекта]

## Реестр задач
| ID | Название | Статус |
|----|----------|--------|
| $1-001 | project-setup | ✅ выполнена |
```

**Шаг 6: Создать init.md**
Создать `E:\$1\.claude\commands\init.md`:

```markdown
# Команда /init — инициализация сессии

Сразу написать: **Начинаю работу**

Вызвать `start_session` с `project_path` = "E:/$1"

Вывести полученный контекст полностью.
→ написать: **Готово**
```

**Шаг 7: Предложить деплой на сервер**
Написать пользователю:
Проект $1 подключён к LocOll:
  - Зарегистрирован в agent-context
  - .mcp.json создан — откройте проект в Claude Code
  - CLAUDE.md создан — базовая конфигурация агента

Хотите развернуть $1 на сервере?
Если есть docker-compose.yml — могу создать директорию на сервере и настроить деплой.
Скажите "да, деплой" или "пропустить".

На этом /onboard завершается. Деплой выполняется только по явной команде.
