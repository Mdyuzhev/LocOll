# LL-002: Fixes — DuckDB WASM загрузка данных, модальные окна для Logs и Terminal

## Цель

Исправить две конкретные проблемы обнаруженные после LL-001:

1. **DuckDB WASM**: ошибка `Catalog Error: Table with name blob:http://... does not exist` — данные загружаются неправильным способом, таблица `metrics` не создаётся.
2. **Logs и Terminal**: открываются inline в строке таблицы вместо всплывающего модального окна. Сделать оба как overlay-модалки поверх страницы.

Дополнительно: добавить горячие клавиши для открытия логов и терминала.

---

## Контекст

Сервер и реквизиты — в `.claude/CLAUDE.md`. **Только paramiko.**

Портал запущен на `http://192.168.1.74:4000`, backend на порту **8010**.
Бэкенд уже отдаёт `/metrics/history` как JSON-массив. Проблема только в клиентской
части — в том как Analytics.templ загружает эти данные в DuckDB WASM.

---

## Проблема 1: DuckDB WASM — неправильная загрузка данных

### Что происходит сейчас

Текущий код делает примерно следующее:

```javascript
// ❌ Неправильно — DuckDB не умеет читать blob: URL как таблицу
const response = await fetch('/metrics/history');
const blob = await response.blob();
const blobUrl = URL.createObjectURL(blob);
await db.query(`CREATE TABLE metrics AS SELECT * FROM '${blobUrl}'`);
```

DuckDB WASM не умеет читать `blob:` URL через SQL — это не файловая система.
Именно это и видно в ошибке: `Table with name blob:http://...`.

### Как правильно

DuckDB WASM имеет метод `registerFileText` / `registerFileBuffer` для регистрации
виртуальных файлов в своей in-memory файловой системе. Нужно зарегистрировать
JSON-данные как именованный виртуальный файл, а затем читать его через SQL.

```javascript
// ✅ Правильно — регистрируем данные как виртуальный файл в DuckDB VFS
const response = await fetch('/metrics/history');
const jsonText = await response.text();

// Регистрируем строку как файл в виртуальной файловой системе DuckDB
await db.registerFileText('metrics.json', jsonText);

// Теперь можно создать таблицу из этого файла
const conn = await db.connect();
await conn.query(`CREATE OR REPLACE TABLE metrics AS SELECT * FROM read_json_auto('metrics.json')`);
```

### Требования к реализации

Исправить `components/analytics.templ` (или соответствующий JS-блок в `handlers/page.go`
если шаблоны инлайн). Логика инициализации DuckDB должна:

1. Импортировать DuckDB WASM используя ES module bundles от `@duckdb/duckdb-wasm`:
   ```javascript
   // CDN-путь для ESM bundle
   import * as duckdb from 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-esm.js';
   ```
   Или использовать UMD bundle если ESM не работает в текущем контексте (Alpine.js
   не всегда дружит с ESM import внутри `x-init`):
   ```html
   <script src="https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-browser-blocking.js"></script>
   ```

2. Инициализировать DuckDB с явным указанием CDN для WASM-файлов:
   ```javascript
   const JSDELIVR_BUNDLES = duckdb.selectBundle({
     mvp: {
       mainModule: 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-mvp.wasm',
       mainWorker: 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-browser-mvp.worker.js',
     },
     eh: {
       mainModule: 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-eh.wasm',
       mainWorker: 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm/dist/duckdb-browser-eh.worker.js',
     },
   });
   const bundle = await JSDELIVR_BUNDLES;
   const worker = new Worker(bundle.mainWorker);
   const logger = new duckdb.ConsoleLogger();
   const db = new duckdb.AsyncDuckDB(logger, worker);
   await db.instantiate(bundle.mainModule);
   ```

3. После инициализации — загрузить данные через `registerFileText` как описано выше.

4. Хранить `db` и `conn` в Alpine.js data объекте чтобы повторные запросы не
   пересоздавали соединение:
   ```javascript
   // В Alpine компоненте
   return {
     db: null,
     conn: null,
     initialized: false,
     async initDuckDB() {
       if (this.initialized) return;
       // ... инициализация ...
       this.initialized = true;
     },
     async runQuery(sql) {
       await this.initDuckDB();
       const result = await this.conn.query(sql);
       // result.toArray() → массив объектов
       return result.toArray();
     }
   }
   ```

5. Формат ответа `/metrics/history` должен быть JSON-массив объектов с полями
   `ts`, `cpu_pct`, `ram_used_mb`, `ram_total_mb`, `disk_used_gb`, `load_avg_1`.
   Проверить что бэкенд отдаёт именно массив `[{...}, {...}]`, а не обёртку
   `{"data": [...]}` — если обёртка есть, DuckDB нужно указывать путь через
   `$.data` в `read_json_auto`.

6. После успешной загрузки показать в UI сколько записей загружено:
   `Loaded N rows (last 30 days)`.

7. Ошибки отображать в UI, не только в консоли. Обернуть всё в try/catch и
   записывать текст ошибки в Alpine-переменную `error` которая показывается
   красным блоком под кнопкой Run Query.

---

## Проблема 2: Logs и Terminal — модальные окна

### Что нужно

При нажатии кнопки "Logs" или "Terminal" в строке контейнера открывается
**overlay-модалка** поверх всей страницы, а не разворачивается inline под строкой.

Требования к модалке:
- Занимает 80% ширины и 80% высоты экрана, центрирована.
- Тёмный полупрозрачный backdrop (`rgba(0,0,0,0.7)`) позади.
- Заголовок: иконка + название контейнера + кнопка закрытия (✕).
- Закрывается по: клику на ✕, клику на backdrop, нажатию `Escape`.
- Горячие клавиши для открытия (см. раздел ниже).
- Модалка одна на всю страницу — не создавать отдельный DOM-элемент на каждую
  строку. Один `<div id="modal-overlay">` в layout, Alpine.js управляет его
  содержимым и видимостью.

### Архитектура модалки (Alpine.js store)

Использовать Alpine.js `$store` для глобального состояния модалки — это позволит
любой строке таблицы открыть модалку без дублирования кода:

```javascript
// В layout.templ, один раз при загрузке страницы
document.addEventListener('alpine:init', () => {
  Alpine.store('modal', {
    open: false,
    type: null,          // 'logs' | 'terminal'
    containerId: null,
    containerName: null,

    show(type, id, name) {
      this.type = type;
      this.containerId = id;
      this.containerName = name;
      this.open = true;
    },

    close() {
      this.open = false;
      // Дать время анимации закрытия (если есть) перед сбросом
      setTimeout(() => {
        this.type = null;
        this.containerId = null;
        this.containerName = null;
      }, 200);
    }
  });
});

// Закрытие по Escape — глобальный обработчик
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') Alpine.store('modal').close();
});
```

В layout.templ добавить один overlay-элемент:

```html
<!-- Один модальный overlay на всю страницу -->
<div
  x-data
  x-show="$store.modal.open"
  x-transition:enter="transition ease-out duration-200"
  x-transition:enter-start="opacity-0"
  x-transition:enter-end="opacity-100"
  @click.self="$store.modal.close()"
  style="position:fixed;inset:0;background:rgba(0,0,0,0.7);z-index:1000;display:flex;align-items:center;justify-content:center;"
  x-cloak>

  <div style="background:#1e1e2e;width:80vw;height:80vh;border-radius:8px;display:flex;flex-direction:column;overflow:hidden;">

    <!-- Заголовок -->
    <div style="padding:12px 16px;border-bottom:1px solid #333;display:flex;justify-content:space-between;align-items:center;">
      <span x-text="($store.modal.type === 'logs' ? '📋 Logs: ' : '⌨️ Terminal: ') + $store.modal.containerName"></span>
      <button @click="$store.modal.close()" style="background:none;border:none;color:#fff;font-size:1.2rem;cursor:pointer;">✕</button>
    </div>

    <!-- Содержимое — зависит от типа -->
    <div style="flex:1;overflow:hidden;padding:8px;">
      <template x-if="$store.modal.type === 'logs'">
        <div x-data="logsPanel()" x-init="init($store.modal.containerId)" style="height:100%;overflow-y:auto;font-family:monospace;font-size:13px;"></div>
      </template>
      <template x-if="$store.modal.type === 'terminal'">
        <div x-data="terminalPanel()" x-init="init($store.modal.containerId)" id="terminal-mount" style="height:100%;"></div>
      </template>
    </div>

  </div>
</div>
```

### Кнопки в строке контейнера

Кнопки Logs и Terminal теперь просто вызывают `$store.modal.show(...)`:

```html
<!-- В container_row.templ -->
<button
  @click="$store.modal.show('logs', '{ containerID }', '{ containerName }')"
  class="btn-info">
  Logs
</button>

<button
  @click="$store.modal.show('terminal', '{ containerID }', '{ containerName }')"
  class="btn-secondary">
  Terminal
</button>
```

### Logs панель внутри модалки

`logsPanel()` Alpine-компонент подключается к SSE-стриму логов контейнера:

```javascript
function logsPanel() {
  return {
    lines: [],
    es: null,
    init(containerId) {
      const el = this.$el;
      this.es = new EventSource(`/containers/${containerId}/logs`);
      this.es.onmessage = (e) => {
        const line = document.createElement('div');
        line.textContent = e.data;
        el.appendChild(line);
        el.scrollTop = el.scrollHeight;  // автоскролл вниз
      };
    },
    destroy() {
      if (this.es) this.es.close();
    }
  }
}
```

`x-init="init(...)"` вызывается когда `<template x-if>` становится видимым.
`destroy()` вызывается Alpine.js автоматически при удалении элемента из DOM
(когда модалка закрывается и `x-if` скрывает элемент).

### Terminal панель внутри модалки

`terminalPanel()` — то же что было в `terminal.templ`, но теперь монтируется
в `#terminal-mount` внутри модалки:

```javascript
function terminalPanel() {
  return {
    term: null,
    ws: null,
    init(containerId) {
      const term = new Terminal({ cursorBlink: true, theme: { background: '#1e1e2e' } });
      const fitAddon = new FitAddon.FitAddon();
      term.loadAddon(fitAddon);
      term.open(this.$el);
      fitAddon.fit();
      const ws = new WebSocket(`ws://${location.host}/terminal/${containerId}`);
      ws.binaryType = 'arraybuffer';
      ws.onmessage = (e) => term.write(new Uint8Array(e.data));
      term.onData(data => ws.readyState === WebSocket.OPEN && ws.send(data));
      this.term = term;
      this.ws = ws;
    },
    destroy() {
      if (this.ws) this.ws.close();
      if (this.term) this.term.dispose();
    }
  }
}
```

---

## Проблема 3: горячие клавиши

Добавить глобальные горячие клавиши для быстрого открытия логов и терминала
**выделенного контейнера** (того на строке которого последний раз навёл курсор пользователь).

| Клавиша | Действие |
|---------|---------|
| `L` | Открыть Logs для активного контейнера |
| `T` | Открыть Terminal для активного контейнера |
| `Escape` | Закрыть модалку |
| `R` | Restart активного контейнера (confirm через Alpine) |

**Активный контейнер** — устанавливается при `mouseenter` на строку таблицы.
Хранится в `Alpine.store('focus')`:

```javascript
Alpine.store('focus', {
  containerId: null,
  containerName: null,
  set(id, name) {
    this.containerId = id;
    this.containerName = name;
  }
});

document.addEventListener('keydown', (e) => {
  // Не реагировать если фокус в textarea, input или открытой модалке-терминале
  const tag = document.activeElement?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA') return;
  if (Alpine.store('modal').type === 'terminal') return;

  const { containerId, containerName } = Alpine.store('focus');
  if (!containerId) return;

  if (e.key === 'l' || e.key === 'L') {
    Alpine.store('modal').show('logs', containerId, containerName);
  }
  if (e.key === 't' || e.key === 'T') {
    Alpine.store('modal').show('terminal', containerId, containerName);
  }
});
```

В строке контейнера добавить `@mouseenter`:

```html
<tr @mouseenter="$store.focus.set('{ containerID }', '{ containerName }')">
  ...
</tr>
```

Визуальная подсказка: активная строка подсвечивается тонкой рамкой или слегка другим фоном.

---

## Порядок выполнения

```
1. Исправить DuckDB WASM загрузку (analytics компонент)
   → Run Query на default SQL работает без ошибок → видна таблица с данными

2. Добавить Alpine.store('modal') и overlay в layout
   → console: Alpine.store('modal') существует

3. Переделать container_row кнопки под store.modal.show(...)
   → клик Logs открывает модалку с заголовком и SSE-стримом

4. Переделать Terminal под модальный вид
   → клик Terminal открывает модалку с xterm.js, bash работает

5. Добавить Alpine.store('focus') и горячие клавиши
   → навести на строку + нажать L → открываются логи

6. Деплой
   → docker compose up -d --build
   → проверить все критерии
```

---

## Критерии готовности

| Проверка | Как убедиться |
|----------|--------------|
| DuckDB загружает данные | Analytics → Run Query с default SQL → таблица с данными, не ошибка |
| DuckDB показывает count | После загрузки: "Loaded N rows" |
| DuckDB ошибки в UI | Намеренно сломать SQL → красный блок с текстом ошибки |
| Logs открывает модалку | Кнопка Logs → overlay поверх страницы с заголовком "📋 Logs: {name}" |
| Logs стримит данные | В модалке появляются строки логов в реальном времени |
| Terminal открывает модалку | Кнопка Terminal → overlay с xterm.js |
| Terminal работает | Ввести `ls` в терминале → видны файлы контейнера |
| Escape закрывает | Нажать Esc → модалка закрывается |
| Backdrop закрывает | Клик вне модалки → закрывается |
| Hotkey L | Навести на строку → нажать L → открываются логи |
| Hotkey T | Навести на строку → нажать T → открывается терминал |
| Hotkey не срабатывает в input | Открыть DuckDB textarea → нажать L → модалка не открывается |
| SSE закрывается | Открыть/закрыть Logs несколько раз → нет утечки EventSource (DevTools → Network) |
| WS закрывается | Открыть/закрыть Terminal → нет утечки WebSocket соединений |

---

## Запрещено

- Создавать отдельный DOM-элемент модалки на каждую строку таблицы — только один глобальный overlay.
- Оставлять SSE/WebSocket соединения открытыми после закрытия модалки — обязательно вызывать `close()`/`dispose()` в `destroy()`.
- Тянуть новые npm-пакеты — только CDN.
- Менять бэкенд если проблема только во фронтенде — сначала проверить формат `/metrics/history`.

---

## ✅ Статус: ВЫПОЛНЕНА

**Дата завершения:** 2026-03-11

**Что сделано:**
- Исправлена загрузка данных DuckDB WASM: заменён blob URL на `registerFileText` + `read_json_auto`
- Добавлен показ количества загруженных строк ("Loaded N rows")
- Добавлен глобальный `Alpine.store('modal')` с единым overlay для Logs и Terminal
- Реализована модалка 80x80vh с backdrop, заголовком, кнопкой закрытия
- Logs панель — SSE EventSource с автоскроллом, закрытие при dismiss
- Terminal панель — xterm.js + WebSocket, dispose при dismiss
- Добавлен `Alpine.store('focus')` с mouseenter на строках контейнеров
- Горячие клавиши: L=Logs, T=Terminal, Escape=закрыть модалку
- Hotkeys не срабатывают в input/textarea/select и при открытой модалке
- Добавлены CSS стили для модалки и подсветки фокусной строки

**Отклонения от плана:**
- Выполнено в соответствии с планом

---

*Дата создания: 2026-03-10*
