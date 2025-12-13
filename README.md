# Антиплагиат — Микросервисная система

**Автор:** Качаев Дмитрий

---

## Стек технологий

- Go 1.21, Docker, docker-compose, HTTP/REST
- Qdrant (векторная БД)
- QuickChart API (генерация облаков слов)

---

## Архитектура

**Компоненты:**

1. **API Gateway** (порт 8000) — маршрутизация запросов
2. **File Storing** (порт 8001) — хранение файлов в `/files`
3. **File Analysis** (порт 8002) — анализ, векторизация, поиск плагиата
4. **Qdrant** (порт 6333) — базаданные с векторами (384-мерные)

**Поток данных:**
```
Клиент → Gateway → File Storing + File Analysis → Qdrant
```

---

## Структура кода

Каждый микросервис разбит на модули (< 200 строк):

**gateway/**
- main.go (37 строк) — инициализация сервера
- handlers.go (103 строки) — /api/submit, /api/works/{id}/reports

**file_storing/**
- main.go (32 строки) — инициализация сервера
- handlers.go (61 строка) — /upload, /files/
- storage.go (55 строк) — файловые операции

**file_analysis/**
- main.go (54 строки) — инициализация сервера
- handlers.go (164 строки) — /analyze, /reports/, /health
- vector.go (34 строки) — генерация 384-мерного вектора (SHA-1)
- plagiarism.go (189 строк) — сохранение/поиск в Qdrant
- qdrant.go (90 строк) — инициализация коллекции
- report.go (41 строка) — формирование JSON-отчётов
- wordcloud.go (106 строк) — генерация PNG облаков слов (QuickChart API)

---

## Процесс проверки

1. Клиент загружает файл через `POST /api/submit` (sender, work_id)
2. Gateway сохраняет файл в File Storing
3. File Analysis читает файл, генерирует вектор, сохраняет в Qdrant
4. Поиск похожих работ среди того же задания (исключая своего автора)
5. Порог плагиата: similarity > 0.95
6. Отчёт сохраняется в `/files/reports/{work_id}/`
7. Ответ клиенту: PNG облако слов + JSON с similarity

---

## Быстрый старт

```bash
docker-compose up --build
```

После старта:
- Gateway доступен по http://localhost:8000
- Все сервисы автоматически подключаются друг к другу

---

## Тестирование с Postman

### Подготовка

1. Скачайте Postman: https://www.postman.com/downloads/
2. Импортируйте `postman_collection.json`

### Запросы в коллекции

**Загрузка работ:** POST /api/submit
- form-data: `file` (файл), `sender` (ФИО), `work_id` (ID задания)
- Ответ: PNG облако слов

**Просмотр отчётов:** GET /api/works/{work_id}/reports
- Параметры: work_id (hw1, hw2, hw3, etc.)
- Ответ: JSON массив отчётов

### Структура отчёта

```json
{
  "file_name": "document.txt",
  "sender": "Иванов Иван",
  "work_id": "hw1",
  "plagiarized": true,
  "similarity": 0.95,
  "timestamp": "2025-12-14T10:30:00Z"
}
```

### Сценарий тестирования

1. **Загрузите две похожие работы для hw1:**
   - POST /api/submit (Ivanov) — файл с текстом "Мама мыла раму"
   - POST /api/submit (Petrov) — файл с тем же текстом

2. **Получите отчёты:**
   - GET /api/works/hw1/reports

3. **Проверьте результаты:**
   - Отчёт Petrov: plagiarized=true, similarity≈1.0
   - Отчёт Ivanov: plagiarized=false, similarity=0.0

---

## API эндпоинты

**Через Gateway (8000):**
- `POST /api/submit` — загрузка и анализ
- `GET /api/works/{work_id}/reports` — получение отчётов
- `GET /health` — проверка статуса

**Прямые (для отладки):**
- File Storing: `POST /upload`, `GET /files/{filename}`, `GET /health`
- File Analysis: `POST /analyze`, `GET /reports/{work_id}`, `GET /health`

---

**Дата:** 12 декабря 2025
