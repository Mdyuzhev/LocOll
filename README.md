# LocOll — Локальный ИИ-ассистент

Прототип для тестирования локальных LLM (1.5B / 7B) в Docker.
Данные не покидают устройство — zero cloud.

---

## 📦 Что нужно для запуска

### Софт
- **Docker Desktop** — [скачать](https://www.docker.com/products/docker-desktop/)
  - После установки запустить Docker Desktop и дождаться зелёной иконки в трее
  - Windows: включить WSL 2 (Docker предложит при установке)
- **Интернет** — для первой загрузки образов и моделей (~4-5 GB)

### Железо (минимум)
| Для модели | RAM | CPU | Диск |
|-----------|-----|-----|------|
| TinyLlama 1.1B | 4 GB | любой x64 | ~1 GB |
| Phi-3 Mini 3.8B | 8 GB | любой x64 | ~3 GB |
| Mistral 7B (**рекомендуем**) | **16 GB** | i5/Ryzen 5+ с AVX2 | ~5 GB |

---

## ⚡ Быстрый старт

### 1. Распакуй архив
```bash
# Распакуй LocOll.zip в любую папку, например:
# Windows: C:\Projects\LocOll
# Mac/Linux: ~/Projects/LocOll
```

### 2. Подними контейнеры
```bash
cd LocOll
docker-compose up -d
```
Первый запуск скачает Docker-образы (~500 MB). Подожди ~30-60 секунд.

### 3. Проверь что всё работает
```bash
docker-compose ps
```
Оба контейнера должны быть `Up`, ollama — `healthy`:
```
locoll-ollama     Up (healthy)
locoll-frontend   Up
```

### 4. Загрузи модель
Через терминал:
```bash
# Рекомендуемая модель (7B, лучшее качество):
docker exec locoll-ollama ollama pull mistral

# Или лёгкая модель (для слабых машин):
docker exec locoll-ollama ollama pull tinyllama
```
Или через UI — нажми кнопку «⬇ Загрузить модель» в интерфейсе.

### 5. Открой в браузере
```
http://localhost:8080
```
Выбери модель в выпадающем списке и начни диалог.

Модели скачиваются один раз и хранятся в volume `locoll-models`.

---

## 🖥️ Требования к железу

### TinyLlama 1.1B (рекомендован для слабых машин)
| Параметр | Значение |
|----------|----------|
| RAM | 4 GB свободных |
| CPU | любой x64 |
| Диск | ~1 GB |
| Скорость | 20–40 tok/s |

### Llama 2 7B (нужен нормальный ноут)
| Параметр | Значение |
|----------|----------|
| RAM | **16 GB** (8 GB — минимум, будет своп) |
| CPU | i5/i7/Ryzen 5+ с **AVX2** поддержкой |
| Диск | ~4 GB |
| Скорость | **4–10 tok/s** (медленно, для демо ок) |

### Проверить AVX2 на Windows:
```
wmic cpu get Caption, Description
# Или в Task Manager → CPU → Instructions
```

---

## 🗂 Структура проекта

```
LocOll/
├── docker-compose.yml     # Оркестрация
└── frontend/
    ├── Dockerfile         # nginx:alpine
    ├── nginx.conf         # Проксируем /api/ → ollama:11434
    └── index.html         # Vue 3 (CDN, без сборки)
```

---

## 🔧 Полезные команды

```bash
# Логи в реальном времени
docker-compose logs -f

# Зайти в контейнер ollama
docker exec -it locoll-ollama bash

# Список загруженных моделей
curl http://localhost:11434/api/tags | python -m json.tool

# Загрузить модель вручную
docker exec locoll-ollama ollama pull llama2

# Удалить модель
docker exec locoll-ollama ollama rm llama2

# Остановить всё
docker-compose down

# Полная очистка (удалит скачанные модели!)
docker-compose down -v
```

---

## 📊 Сравнение моделей

| Модель | Размер | RAM | Скорость | Качество | Use case |
|--------|--------|-----|----------|----------|----------|
| TinyLlama 1.1B | 637 MB | 4 GB | ⚡⚡⚡ | ★★☆ | Тест железа |
| Phi-3 Mini 3.8B | 2.2 GB | 8 GB | ⚡⚡☆ | ★★★ | Лучший компромисс |
| Llama 2 7B | 3.8 GB | 16 GB | ⚡☆☆ | ★★★ | Демо заказчику |
| Mistral 7B | 4.1 GB | 16 GB | ⚡☆☆ | ★★★+ | Лучше Llama 2 |

**Рекомендация для демо**: Mistral 7B > Llama 2 7B по качеству при тех же требованиях.

---

## ⚙️ Настройка

### Сменить порт (если 8080 занят)
В `docker-compose.yml` поменяй порт frontend:
```yaml
ports:
  - "3000:80"  # вместо 8080:80
```
Потом: `docker-compose down && docker-compose up -d`

### Увеличить контекстное окно
По умолчанию `num_ctx: 2048` (экономия RAM). Если RAM >= 16 GB, можно поднять до 4096 в настройках UI или в запросе к API.

### Включить GPU (NVIDIA)
Раскомментируй блок `deploy` в `docker-compose.yml`:
```yaml
deploy:
  resources:
    reservations:
      devices:
        - driver: nvidia
          count: 1
          capabilities: [gpu]
```
Требуется: NVIDIA GPU + [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html).

---

## 🛠 Troubleshooting

**Docker Desktop не запускается / WSL ошибка:**
```bash
# Windows — включить WSL 2:
wsl --install
# Перезагрузить ПК, потом заново запустить Docker Desktop
```

**Контейнер ollama не стартует (unhealthy):**
```bash
docker-compose logs ollama             # смотреть ошибки
docker-compose down && docker-compose up -d  # перезапуск
```

**Модель не загружается:**
```bash
docker exec locoll-ollama ollama list  # проверить список
docker-compose logs ollama             # смотреть ошибки
```

**Медленно работает 7B:**
- Это норма для CPU. 4-8 tok/s = ~1 слово в секунду.
- Для демо: задавайте короткие вопросы с коротким ответом.
- Альтернатива: `phi3:mini` — быстрее при сравнимом качестве.

**Out of memory:**
- Закрыть лишние приложения, освободить RAM.
- Для 8 GB ноута использовать только TinyLlama или Phi-3 Mini.
- Проверить RAM в Docker Desktop: Settings → Resources → Memory (поставить максимум).

**Порт 8080 занят:**
```yaml
# В docker-compose.yml:
ports:
  - "3000:80"  # или любой свободный
```

**Проверить поддержку AVX2 (нужно для 7B моделей):**
```bash
# Windows:
wmic cpu get Caption, Description

# Linux:
grep avx2 /proc/cpuinfo
```

---

## 🔄 Обновление и обслуживание

```bash
# Обновить образы до последних версий
docker-compose pull && docker-compose up -d --build

# Список загруженных моделей
docker exec locoll-ollama ollama list

# Удалить модель (освободить диск)
docker exec locoll-ollama ollama rm mistral

# Остановить без удаления данных
docker-compose down

# Полная очистка (удалит скачанные модели!)
docker-compose down -v
```
