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

## ✅ Статус: ВЫПОЛНЕНА

**Дата завершения:** 2026-03-10

**Что сделано:**
- Go-бэкенд на chi + templ + SQLite запущен на порту 8010 (network_mode: host)
- Nginx на порту 4000 проксирует к бэкенду
- Docker SDK: листинг контейнеров, start/stop/restart
- SSE broker: живые метрики раз в минуту через hx-swap-oob
- PTY + gorilla/websocket: терминал в браузере через xterm.js
- Ollama клиент: AI-анализ контейнеров (tinyllama, mistral)
- DuckDB WASM секция: загружается, но есть баг с blob: URL (исправляется в LL-002)
- Go WASM: форматирование аптайма через goFormatUptime()
- /api/v1/* JSON API для агентов

**Известные проблемы перешедшие в LL-002:**
- DuckDB WASM: ошибка `Catalog Error: Table with name blob:http://...`
- Logs и Terminal открываются inline вместо модальных окон
- Нет горячих клавиш

---

*Дата создания: 2026-03-10*
*Перемещено в done: 2026-03-10*
