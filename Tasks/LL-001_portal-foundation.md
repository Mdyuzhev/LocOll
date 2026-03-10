# LL-001: Portal Foundation — фундамент Lab Portal

## Цель

Построить фундамент homelab-портала: Go-бэкенд с полной инфраструктурой (chi, templ,
SQLite, Docker SDK, SSE, PTY, Ollama), htmx + Alpine.js фронтенд, Go WASM модуль,
DuckDB WASM для аналитики. Результат — работающий портал на порту :4000 с живыми
метриками, управлением контейнерами, терминалом в браузере и AI-анализом через
локальный Ollama.

Это **экспериментальный проект** — приоритет на изучение технологий, не на
production-hardening.

---

## Контекст

Сервер и реквизиты подключения — в `.claude/CLAUDE.md`.

**Подключение к серверу — только через paramiko. Никакого ssh/sshpass.**

### Что уже запущено на сервере

| Сервис | Порт | Примечание |
|--------|------|-----------|
| Ollama | 11434 | Не Docker, отдельный процесс. Использовать модели которые уже загружены — проверить `curl localhost:11434/api/tags` |
| locoll-frontend (старый) | 8085 | Остановить в рамках деплоя |
| warehouse-api | 8080 | Health-check цель |
| warehouse-grafana | 3001 | Health-check цель |
| warehouse-prometheus | 9090 | Health-check цель |
| errorlens-backend | 8002 | Health-check цель |

**Новый портал занимает порт :4000.**

---

## Стек и зависимости

### `go.mod`

```
module locoll

go 1.22

require (
    github.com/go-chi/chi/v5
    github.com/a-h/templ
    modernc.org/sqlite
    github.com/docker/docker
    github.com/creack/pty
)
```

### Frontend (CDN в templ-шаблонах, без npm и сборщиков)

| Библиотека | Источник |
|-----------|---------|
| htmx 2.x | unpkg.com/htmx.org@2 |
| htmx SSE extension | unpkg.com/htmx-ext-sse |
| Alpine.js 3.x | cdn.jsdelivr.net/npm/alpinejs@3/dist/cdn.min.js |
| xterm.js 5.x | cdn.jsdelivr.net/npm/xterm@5 |
| xterm-addon-fit | cdn.jsdelivr.net/npm/xterm-addon-fit |

DuckDB WASM и Go WASM подключаются лениво — только при открытии соответствующей
вкладки через `<script>` внутри Alpine-компонента. Не грузить оба в `<head>`.

---

## Структура проекта

```
~/projects/locoll/
├── cmd/portal/main.go            # точка входа: роутер, запуск горутин
├── internal/
│   ├── collector/
│   │   └── collector.go          # горутина: метрики каждую минуту → SQLite + SSE
│   ├── store/
│   │   ├── store.go              # SQLite init + миграции через schema_version
│   │   ├── metrics.go            # write/read метрик
│   │   └── events.go             # write/read событий контейнеров
│   ├── docker/
│   │   └── client.go             # обёртка Docker SDK
│   ├── system/
│   │   └── system.go             # чтение /proc/* для метрик железа
│   ├── sse/
│   │   └── broker.go             # SSE fan-out broker
│   ├── pty/
│   │   └── pty.go                # PTY сессии для терминала
│   ├── ollama/
│   │   └── client.go             # Ollama API клиент
│   └── handlers/
│       ├── system.go             # /fragments/server, /api/v1/system
│       ├── containers.go         # /fragments/containers, POST /containers/{id}/*
│       ├── logs.go               # /containers/{id}/logs → SSE
│       ├── terminal.go           # WS /terminal/{id} → PTY
│       ├── services.go           # /fragments/services, /api/v1/services
│       ├── metrics.go            # /metrics/history, /api/v1/metrics
│       ├── ai.go                 # /ai/analyze, /api/v1/analyze
│       └── events.go             # /api/v1/events
├── components/
│   ├── layout.templ              # базовый layout
│   ├── server_card.templ         # виджет железа
│   ├── containers.templ          # список проектов
│   ├── container_row.templ       # строка контейнера с кнопками
│   ├── services.templ            # health-check карточки
│   ├── terminal.templ            # xterm.js виджет
│   ├── analytics.templ           # DuckDB WASM секция
│   └── ai_panel.templ            # AI-панель
├── wasm/
│   └── metrics/
│       └── main.go               # Go WASM: форматирование, аптайм
├── static/
│   ├── app.wasm                  # скомпилированный Go WASM (gitignore, собирается при деплое)
│   ├── wasm_exec.js              # Go WASM runtime (cp из GOROOT/misc/wasm/)
│   └── style.css                 # минимальные переопределения
├── nginx/
│   └── nginx.conf
├── Dockerfile
├── docker-compose.yml
└── .air.toml
```

---

## P1: Инфраструктура Go-бэкенда

### `cmd/portal/main.go`

Точка входа делает четыре вещи по порядку: инициализирует SQLite store (DDL +
миграции), создаёт SSE broker и Docker client, запускает горутину-коллектор через
`go collector.Start(ctx, store, dockerClient, sseBroker)`, запускает HTTP-сервер
через chi.

Структура роутера:

```
GET  /                          → full page (layout + все виджеты)
GET  /events                    → SSE endpoint (живые HTML-фрагменты)

GET  /fragments/server          → ServerCard фрагмент
GET  /fragments/containers      → ContainerList фрагмент
GET  /fragments/services        → ServiceGrid фрагмент

POST /containers/{id}/restart   → перезапустить → 200 + обновлённая строка контейнера
POST /containers/{id}/stop      → остановить
POST /containers/{id}/start     → запустить
GET  /containers/{id}/logs      → SSE стрим docker logs

WS   /terminal/{id}             → WebSocket PTY (docker exec -it)

GET  /metrics/history           → JSON метрики из SQLite (для DuckDB WASM)
POST /ai/analyze                → AI анализ через Ollama

GET  /api/v1/health             → {"status":"ok"}
GET  /api/v1/system             → JSON метрики железа
GET  /api/v1/containers         → JSON список контейнеров
GET  /api/v1/services           → JSON health-check
GET  /api/v1/metrics            → JSON история метрик
GET  /api/v1/events             → JSON последние N событий
POST /api/v1/analyze            → JSON AI анализ

GET  /static/*                  → статика
```

### `internal/collector/collector.go`

Горутина `Start(ctx, store, docker, broker)` работает в бесконечном цикле с тиком
каждые 60 секунд. На каждом тике: читает метрики железа через `system.Read()`,
читает статус всех контейнеров через docker SDK, пишет запись в `metrics`, сравнивает
текущие статусы с предыдущими и если что-то изменилось — пишет запись в `events`,
рендерит templ-компонент `ServerCard` и рассылает через `broker.Broadcast()`.

Раз в сутки (отдельный тикер): удаляет записи из `metrics` и `events` старше 30 дней.

При старте горутины — сразу делает первый сбор не дожидаясь минуты.

### `internal/store/store.go`

Открывает SQLite через `modernc.org/sqlite` с DSN
`file:/home/flomaster/projects/locoll/data/portal.db?_journal=WAL`. Режим WAL
обязателен — иначе одновременные читающие запросы будут блокироваться на пишущей
горутине коллектора.

Миграции применяются через таблицу `schema_version`. При каждом старте: читает
текущую версию, применяет недостающие миграции из встроенного массива `migrations[]`
по порядку. Это простой подход без внешних инструментов.

DDL таблиц — в `.claude/CLAUDE.md`, раздел "Хранилище".

### `internal/sse/broker.go`

SSE broker реализует паттерн fan-out. Внутри — `map[chan string]struct{}` клиентов
и мьютекс. Метод `Subscribe()` создаёт канал, регистрирует его и возвращает.
Метод `Unsubscribe(ch)` удаляет канал из map и закрывает его. Метод
`Broadcast(html string)` итерирует по всем каналам и отправляет — неблокирующий
select с `default`, чтобы медленный клиент не замедлял остальных.

Хэндлер `/events` в `handlers/system.go` выставляет заголовки SSE
(`Content-Type: text/event-stream`, `Cache-Control: no-cache`), подписывается на
broker, и в цикле читает из канала и пишет в `ResponseWriter`. При закрытии
соединения (контекст отменён) — отписывается.

### `internal/pty/pty.go`

Функция `NewSession(containerID string)` запускает `docker exec -it {id} /bin/sh`
через `os/exec`, оборачивает процесс в PTY через `creack/pty`, возвращает структуру
с `*os.File` (PTY master) и `*exec.Cmd`.

Хэндлер WebSocket в `handlers/terminal.go` апгрейдит соединение (stdlib
`net/http` умеет это через `hijack`), создаёт PTY-сессию, запускает две горутины:
одна читает из PTY stdout и пишет в WebSocket (бинарные фреймы), другая читает из
WebSocket и пишет в PTY stdin. При закрытии — завершает процесс и освобождает PTY.

**Примечание**: для WebSocket апгрейда в stdlib нужен `http.Hijacker`. Если это
усложняет код — можно добавить `golang.org/x/net/websocket` или `gorilla/websocket`
как зависимость.

### `internal/ollama/client.go`

Структура `Client` с полем `baseURL = "http://localhost:11434"`. Метод
`Generate(ctx, model, prompt string) (string, error)` делает POST на
`/api/generate` с `{"model": model, "prompt": prompt, "stream": false}`.

Метод `ListModels(ctx) ([]string, error)` делает GET `/api/tags` и возвращает
список имён загруженных моделей.

Метод `AnalyzeContainer(ctx, containerName string) (string, error)` — высокоуровневый:
читает из SQLite последние 50 событий контейнера, метрики за последний час,
текущий healthcheck статус через Docker SDK, собирает промпт и вызывает `Generate`.
Промпт на английском (модели лучше работают с английским):

```
You are a DevOps assistant analyzing a home lab server.

Container: {name}
Current status: {status}
Health: {health}

Recent events (last 50):
{events}

Server metrics last hour (CPU%, RAM used GB):
{metrics}

Briefly explain what might be wrong and suggest what to check.
```

---

## P2: Content Negotiation

Вспомогательная функция в пакете handlers:

```go
func wantsJSON(r *http.Request) bool {
    return strings.Contains(r.Header.Get("Accept"), "application/json")
}
```

Каждый хэндлер который обслуживает и браузер и агента использует эту функцию.
`/api/v1/*` роуты — всегда JSON без проверки.

---

## P3: templ компоненты и htmx-паттерны

### `components/layout.templ`

Базовый layout подключает в `<head>` все CDN-скрипты. В `<body>` — навигация
(Server, Containers, Services, Analytics, AI) и основной контент через `@children`.

Ключевой элемент в layout — невидимый div который слушает SSE и управляет
обновлением всей страницы:

```html
<div id="sse-listener"
     hx-ext="sse"
     sse-connect="/events"
     style="display:none">
</div>
```

SSE broker рассылает HTML-фрагменты с `hx-swap-oob="true"` — это Out-of-Band
подмена: htmx находит элемент в DOM по ID и заменяет его, даже если замена пришла
через SSE, а не через обычный hx-get запрос. Это ключевой паттерн всего UI.

### `components/server_card.templ`

Горизонтальная карточка с пятью блоками. Оборачивается в:

```html
<div id="server-metrics" hx-swap-oob="true">
  <!-- содержимое -->
</div>
```

Благодаря этому SSE broker может рассылать этот фрагмент и htmx автоматически
обновит его в DOM. CPU, RAM, Disk — с прогресс-баром и цветовыми порогами:
зелёный < 70%, жёлтый 70–90%, красный > 90%.

### `components/container_row.templ`

Строка контейнера содержит кнопки управления. Они используют htmx POST с
подтверждением через Alpine.js:

```html
<!-- Пример кнопки Restart -->
<button
  x-data="{ confirm: false }"
  @click="confirm = true"
  x-show="!confirm"
  class="btn-warning">
  Restart
</button>
<div x-show="confirm" x-cloak>
  <span>Sure?</span>
  <button
    hx-post="/containers/{ containerID }/restart"
    hx-target="#row-{ containerID }"
    hx-swap="outerHTML"
    @click="confirm = false">
    Yes
  </button>
  <button @click="confirm = false">No</button>
</div>
```

`hx-target="#row-{id}"` и `hx-swap="outerHTML"` — после успешного POST сервер
возвращает обновлённую строку контейнера, htmx заменяет её в DOM. Весь остальной
список не перезагружается.

Иконки статуса: ✅ healthy, 🟢 running (без healthcheck), 🔴 unhealthy, ⬛ exited,
🔵 restarting.

### `components/terminal.templ`

Виджет xterm.js. При клике "Open Terminal" Alpine.js инициализирует xterm и
открывает WebSocket:

```html
<div x-data="terminalApp('{ containerID }')" x-init="init()">
  <div id="terminal-{ containerID }" style="height:400px"></div>
</div>

<script>
function terminalApp(id) {
  return {
    term: null,
    ws: null,
    init() {
      // Загружаем xterm только когда нужен, не в head
      const term = new Terminal({ cursorBlink: true });
      const fitAddon = new FitAddon.FitAddon();
      term.loadAddon(fitAddon);
      term.open(document.getElementById('terminal-' + id));
      fitAddon.fit();

      const ws = new WebSocket(`ws://${location.host}/terminal/${id}`);
      ws.onmessage = e => term.write(e.data);
      term.onData(data => ws.send(data));
      this.term = term;
      this.ws = ws;
    }
  }
}
</script>
```

### `components/analytics.templ`

Секция для DuckDB WASM. Содержит текстовое поле для SQL-запроса и таблицу результатов.
Alpine.js управляет загрузкой DuckDB WASM (один раз при первом открытии секции)
и выполнением запросов. Данные загружаются с `/metrics/history` (JSON), DuckDB WASM
регистрирует их как in-memory таблицу `metrics`.

Пример который показывается по умолчанию:
```sql
SELECT
  datetime(ts, 'unixepoch') as time,
  round(cpu_pct, 1) as cpu,
  round(ram_used_mb / 1024.0, 2) as ram_gb
FROM metrics
WHERE ts > unixepoch('now') - 3600
ORDER BY ts DESC
LIMIT 20
```

### `components/ai_panel.templ`

Форма с выпадающим списком контейнеров (заполняется данными из Docker SDK) и
кнопкой "Analyze". Нажатие делает htmx POST на `/ai/analyze` с `container_id` в теле.
Сервер возвращает HTML-фрагмент с текстом ответа от Ollama. Пока Ollama думает —
показывать спиннер через `hx-indicator`.

---

## P4: Go WASM модуль

### `wasm/metrics/main.go`

Go WASM экспортирует JavaScript-функции через `//export` + `js/syscall`:

```go
//go:build js,wasm

package main

import (
    "fmt"
    "syscall/js"
    "time"
)

// FormatUptime принимает секунды и возвращает "3 days 2 hours 15 min"
func formatUptime(this js.Value, args []js.Value) interface{} {
    seconds := args[0].Int()
    d := time.Duration(seconds) * time.Second
    days := int(d.Hours()) / 24
    hours := int(d.Hours()) % 24
    mins := int(d.Minutes()) % 60
    return fmt.Sprintf("%d days %d hours %d min", days, hours, mins)
}

func main() {
    js.Global().Set("goFormatUptime", js.FuncOf(formatUptime))
    // Блокируем горутину main — иначе WASM завершится сразу
    select {}
}
```

Компиляция: `GOOS=js GOARCH=wasm go build -o static/app.wasm ./wasm/metrics/`

В Dockerfile это запускается как отдельный build-stage и артефакт копируется в `static/`.

`wasm_exec.js` — копируется из `$(go env GOROOT)/misc/wasm/wasm_exec.js` в `static/`
командой в Dockerfile.

### Подключение в layout.templ

```html
<!-- Загружаем WASM асинхронно, не блокируем страницу -->
<script src="/static/wasm_exec.js"></script>
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('/static/app.wasm'), go.importObject)
    .then(result => go.run(result.instance));
</script>
```

---

## P5: Docker Compose и конфигурация

### `docker-compose.yml`

```yaml
services:
  backend:
    build:
      context: .
      dockerfile: Dockerfile
      target: backend
    network_mode: host
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /proc:/proc:ro
      - portal_data:/home/flomaster/projects/locoll/data
    restart: unless-stopped

  nginx:
    build:
      context: .
      dockerfile: Dockerfile
      target: nginx
    ports:
      - "4000:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  portal_data:
```

`backend` в `network_mode: host` — поэтому nginx проксирует не на `backend:8000`
а на `localhost:8000`.

### `nginx/nginx.conf`

```nginx
server {
    listen 80;

    location /events {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Connection '';       # обязательно для SSE
        proxy_buffering off;                  # обязательно для SSE
        proxy_cache off;
        proxy_read_timeout 3600s;
    }

    location /terminal/ {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;   # обязательно для WebSocket
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }

    location /api/ {
        proxy_pass http://localhost:8000;
    }

    location / {
        proxy_pass http://localhost:8000;
    }
}
```

Здесь нет раздачи статики через nginx — Go-бэкенд сам обслуживает `/static/*`
через `http.FileServer`. Это проще и не требует копирования файлов между stages.

### `Dockerfile` — multi-stage

```dockerfile
# Stage 1: сборка Go бинарника + WASM
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Сборка основного бинарника
RUN go build -o portal ./cmd/portal/
# Сборка WASM
RUN GOOS=js GOARCH=wasm go build -o static/app.wasm ./wasm/metrics/
# Копирование WASM runtime из Go installation
RUN cp $(go env GOROOT)/misc/wasm/wasm_exec.js static/

# Stage 2: backend образ
FROM alpine:3.19 AS backend
WORKDIR /app
COPY --from=builder /app/portal .
COPY --from=builder /app/static ./static
CMD ["./portal"]

# Stage 3: nginx образ
FROM nginx:alpine AS nginx
COPY nginx/nginx.conf /etc/nginx/conf.d/default.conf
```

### `.air.toml` (для разработки)

```toml
[build]
cmd = "go build -o ./tmp/portal ./cmd/portal/"
bin = "./tmp/portal"
include_ext = ["go", "templ"]
exclude_dir = ["tmp", "static", "wasm"]

[misc]
clean_on_exit = true
```

---

## P6: Деплой

### Последовательность (через paramiko)

```python
cmds = [
    # 1. Создать структуру директорий
    ('mkdir', 'mkdir -p ~/projects/locoll/data ~/projects/locoll/nginx ~/projects/locoll/static'),

    # 2. Остановить старый locoll-frontend
    ('stop-old', 'docker stop locoll-frontend 2>/dev/null || true'),

    # 3. Передать файлы через SFTP (все файлы проекта)
    # ... SFTP uploads ...

    # 4. Собрать и запустить
    ('build', 'cd ~/projects/locoll && docker compose up -d --build 2>&1 | tail -40'),

    # 5. Проверить что контейнеры запустились
    ('check', 'docker ps --format "{{.Names}}\t{{.Status}}" | grep locoll'),

    # 6. Проверить API
    ('health', 'curl -s http://localhost:4000/api/v1/health'),

    # 7. Проверить что Ollama доступна из backend
    ('ollama', 'curl -s http://localhost:11434/api/tags | python3 -c "import sys,json; tags=json.load(sys.stdin); print([m[\'name\'] for m in tags.get(\'models\',[])])"'),
]
```

---

## Тесты

Минимальный набор — это инструментальный проект, не продуктовый.

| Файл | Что проверяет |
|------|---------------|
| `internal/store/store_test.go` | Запись и чтение метрик, миграции идемпотентны |
| `internal/sse/broker_test.go` | Broadcast доходит до подписчиков, медленный клиент не блокирует |
| `internal/ollama/client_test.go` | Формирование промпта с контекстом из SQLite |
| `internal/handlers/containers_test.go` | Content negotiation: Accept: application/json → JSON, иначе HTML |

---

## Критерии готовности

| Проверка | Как убедиться |
|----------|--------------|
| Портал открывается | http://192.168.1.74:4000 → страница с данными, не пустая |
| Метрики живые | ServerCard обновляется раз в минуту без перезагрузки |
| Контейнеры сгруппированы | WarehouseHub, ErrorLens, LocOll в отдельных карточках |
| Управление работает | Кнопка Restart на контейнере → строка обновляется, лишнее не перегружается |
| Логи через SSE | Клик "Logs" → появляется стрим логов в реальном времени |
| Терминал работает | Клик "Terminal" → xterm.js открывается, команды выполняются внутри контейнера |
| AI анализ | Выбрать unhealthy контейнер → нажать Analyze → получить ответ от Ollama |
| DuckDB WASM | Открыть Analytics → вставить SQL по таблице `metrics` → увидеть результат |
| Go WASM | Uptime на ServerCard отображается через `goFormatUptime()` из WASM |
| JSON API | `curl -H "Accept: application/json" localhost:4000/api/v1/containers` → валидный JSON |
| История накапливается | Через час в SQLite есть ~60 записей в таблице metrics |
| Старый locoll остановлен | `docker ps` не показывает locoll-frontend на :8085 |

---

## Порядок выполнения

Строго по этапам — каждый должен компилироваться и запускаться перед переходом к следующему.

```
P1-a: go.mod + store (SQLite init + миграции) + system (читает /proc)
       → убедиться что `go build ./...` проходит

P1-b: collector + SSE broker + docker client
       → убедиться что горутина стартует и пишет в SQLite

P1-c: handlers (system, containers, services) без UI
       → curl /api/v1/system, /api/v1/containers возвращают JSON

P3:   templ компоненты + layout + htmx паттерны
       → страница открывается, ServerCard обновляется через SSE

P1-d: PTY + terminal handler + WebSocket
       → xterm.js соединяется, bash работает

P3-ai: ai_panel.templ + ollama handler
       → анализ контейнера возвращает ответ

P4:   Go WASM модуль + подключение в layout
       → goFormatUptime доступна в консоли браузера

P5-duckdb: analytics.templ + DuckDB WASM
       → SQL запрос к metrics работает в браузере

P6:   docker-compose.yml + Dockerfile + деплой на сервер
       → все критерии готовности пройдены
```

---

## Запрещено

- Использовать `ssh`, `sshpass` — только paramiko
- Использовать `kubectl` — K3s удалён
- Добавлять PostgreSQL, Redis — только SQLite
- Тянуть новые модели Ollama — только те что уже загружены на сервере
- Хардкодить список контейнеров — только через Docker label `com.docker.compose.project`
- npm, yarn, любые JS-сборщики — только CDN для фронтенда
- `psutil` или внешние Go-библиотеки для метрик — только `/proc/*` напрямую

---

*Дата создания: 2026-03-10*
