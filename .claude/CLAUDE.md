# CLAUDE.md — Агент проекта LocOll (Lab Portal)

## Начало работы

При открытии нового чата — запустить `/init`.
Для полного среза сервера — `/server-status`.

## 🔄 ЧЕКПОИНТЫ — обязательно

Контекст разговора конечен и сжимается без предупреждения. Чтобы не терять прогресс:

1. **При завершении каждого todo** — вызвать `mcp__agent-context__checkpoint` с описанием что сделано.
2. **Каждые 5 вызовов инструментов** — принудительный checkpoint.
3. **При получении system-reminder о сжатии контекста** — немедленный checkpoint.
4. **Перед завершением задачи** — `end_session` с полным итогом.

Формат checkpoint: `"[шаг N] краткое описание — что сделано, что дальше"`

Если `agent-context` MCP недоступен — предупредить и продолжить без него.

---

## Что это за проект

**LocOll** — экспериментальный homelab-портал. Единый дашборд для управления и
мониторинга домашней лаборатории: контейнеры, метрики, логи, аналитика, терминал, AI.

Проект **экспериментальный** — полигон для изучения Go, WebAssembly, htmx, SSE, DuckDB WASM.

**Владелец**: Flomaster (Михаил)
**GitHub**: `https://github.com/Mdyuzhev/LocOll`

---

## 🔴 КРИТИЧНО: КАК ПОДКЛЮЧАТЬСЯ К СЕРВЕРУ

**Голый `ssh` и `sshpass` НЕ РАБОТАЮТ.** Среда агента — Windows с кириллическим
именем пользователя. Ломается путь к `~/.ssh/known_hosts`.

**Единственный правильный способ — Python + paramiko. Всегда. Без исключений.**

```python
import paramiko, sys
sys.stdout.reconfigure(encoding='utf-8')
client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.1.74', username='flomaster', password='Misha2021@1@', timeout=10)
_, stdout, stderr = client.exec_command('КОМАНДА')
print(stdout.read().decode('utf-8', errors='replace').strip())
client.close()
```

| Параметр | Значение |
|----------|----------|
| **Host** | **192.168.1.74** (LAN — основной) / 100.81.243.12 (Tailscale — удалённо) |
| User | flomaster |
| Password | Misha2021@1@ |
| Путь на сервере | /opt/locoll |

Ключевые порты: Portal **4000**, backend **8010**, homelab-mcp **8765**, Ollama **11434**.
Полные шаблоны — `E:\HomeLab\server-access.md`. Полная карта — `E:\HomeLab\server_map.md`.

---

## ✅ Homelab MCP — stateless режим (LL-015)

**Проблема устранена навсегда.** Ранее сессии homelab-mcp протухали через 10-15 минут
неактивности, что ломало все инструменты до перезапуска чата.

**Решение (коммит 1d724a9, 2026-03-16):** в `mcp-server/server.py` добавлен флаг:

```python
mcp.run(transport="streamable-http", host="0.0.0.0", port=8765, stateless_http=True)
```

В stateless режиме каждый HTTP POST обрабатывается независимо — никаких сессий,
никаких mcp-session-id, никаких таймаутов. Сервер отвечает `{"status":"ok"}`,
заголовок `mcp-session-id` в ответах отсутствует.

---

Полный справочник (стек, архитектура, структура, команды): `.claude/reference.md`

---

## ⚠️ Известные проблемы

- **CI runner**: GitHub Actions деплоит в `/opt/locoll`. При push: down → build --no-cache → up.
- **Go WASM toolchain**: Docker скачивает Go 1.25 toolchain — увеличивает время сборки.
- ~~**PM2 autostart**~~: Удалён (LL-016). agent-context теперь в Docker Desktop с `restart: unless-stopped`.

---

## 📋 Реестр задач

| ID | Название | Статус |
|----|----------|--------|
| LL-001 | portal-foundation | ✅ выполнена |
| LL-002 | duckdb-fix-and-modals | ✅ выполнена |
| LL-003 | mcp-server-homelab | ✅ выполнена |
| LL-004 | agent-context-mcp | ✅ выполнена |
| LL-005 | server-mcp-bridge | ✅ выполнена |
| LL-006 | homelab-telegram-bot | ✅ выполнена |
| LL-007 | telegram-notes-to-agent | ✅ выполнена |
| LL-008 | claude-bridge-endpoint | ✅ выполнена |
| LL-009 | portal-improvements | ✅ выполнена |
| LL-010 | homelab-proxy-stability | ✅ выполнена |
| LL-011 | init-speed-optimization | ✅ выполнена |
| LL-012 | startup-speed | ✅ выполнена |
| LL-013 | agent-context-http | ✅ выполнена |
| LL-014 | pm2-windows-autostart | ✅ выполнена |
| LL-015 | homelab-mcp-stateless | ✅ выполнена |
| LL-016 | agent-context-python | ✅ выполнена |

Файлы задач: `E:\LocOll\Tasks\backlog\LL-NNN_slug.md` (в работе), `E:\LocOll\Tasks\done\LL-NNN_slug.md` (выполненные)

---

## 🚫 Запрещено

- `ssh`, `sshpass` — только paramiko
- `kubectl` — K3s удалён с сервера
- PostgreSQL, Redis — только SQLite
- npm build на сервере — только Docker multi-stage
- Хардкодить список контейнеров — только через Docker label `com.docker.compose.project`
- Тянуть новые модели Ollama — использовать только те что загружено

---

*Последнее обновление: 2026-03-17 (LL-016 выполнена — agent-context на Python/Docker, PM2 удалён)*
