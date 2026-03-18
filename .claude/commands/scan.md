# Команда /scan — обнаружение новых проектов на диске E:\

Сразу написать: **Сканирую E:\ на незарегистрированные проекты...**

**Шаг 1: Получить список зарегистрированных проектов**
Вызвать `scan_projects(root="E:/", check_registry=True)`.
Сохранить `registered_paths` из ответа.

**Шаг 2: Просканировать E:\**
Прочитать содержимое `E:\` (через Glob или ls).
Отфильтровать системные и нерелевантные директории (см. SKIP_DIRS в production.py).

**Шаг 3: Для каждой потенциальной директории**
Проверить наличие маркеров проекта (.git, docker-compose.yml, Dockerfile, package.json, go.mod, requirements.txt).
Если маркеры найдены и путь НЕ в registered_paths — вызвать `identify_project(path, git_remote)`.
git_remote получить из `.git/config` если есть.

**Шаг 4: Вывести отчёт**
Формат:

```
## Результат сканирования E:\

### Зарегистрированные проекты (N):
- LocOll — experiment — E:/LocOll
- ...

### Новые проекты (M):
- **ProjectName** — тип: bot — E:/ProjectName
  Remote: git@github.com:user/repo.git
  → `/onboard ProjectName` чтобы подключить

### Пропущены (не проекты):
- $RECYCLE.BIN, System Volume Information, ...
```

Если новых проектов нет — написать "Все проекты на E:\ уже зарегистрированы."
