# Антиплагиат — Микросервисная система

**Автор:** Качаев Дмитрий

---

## Используемый стек
- **Язык программирования**: Go (Golang 1.21)
- **Контейнеризация и запуск**: Docker, docker-compose
- **Базы данных**: Qdrant (векторная БД для поиска схожести работ)
- **API-интерфейсы и коммуникации**: HTTP/REST, стандартные библиотеки net/http, encoding/json
- **Тестирование и демонстрация**: curl, jq (для разбора JSON-вывода), bash-скрипты

---

## Назначение
Решение предназначено для автоматизации приёма и проверки студенческих работ на оригинальность. Архитектура построена по принципу ответственности микросервисов и рассчитана на масштабирование и простоту сопровождения.

---

## Архитектура системы

### Компоненты системы

1. **API Gateway (порт 8000)**
   - Единая точка входа для всех клиентских запросов
   - Маршрутизация запросов к соответствующим микросервисам
   - Агрегация ответов от нескольких сервисов
   - Обработка ошибок и возврат понятных сообщений клиенту

2. **File Storing Service (порт 8001)**
   - Приём и сохранение файлов работ студентов
   - Выдача сохранённых файлов по запросу
   - Использует Docker volume для персистентного хранения

3. **File Analysis Service (порт 8002)**
   - Генерация векторных представлений (эмбеддингов) из текста работ
   - Сохранение векторов в Qdrant
   - Поиск похожих работ через векторный поиск
   - Формирование отчётов о плагиате
   - Сохранение отчётов в файловой системе

4. **Qdrant Vector Database (порт 6333)**
   - Хранение векторных представлений работ
   - Быстрый поиск похожих векторов по косинусной метрике
   - Фильтрация по метаданным (work_id, sender)

### Принципы архитектуры
- **Разделение ответственности**: каждый сервис решает одну задачу
- **Независимость сервисов**: сервисы могут масштабироваться отдельно
- **Синхронная коммуникация**: взаимодействие через HTTP REST API
- **Обработка ошибок**: каждый сервис корректно обрабатывает недоступность других сервисов

---

## Технические сценарии взаимодействия между сервисами

### Сценарий 1: Загрузка работы студентом

**Пользовательский сценарий:** Студент отправляет работу через API Gateway.

**Технический сценарий обмена данными:**

1. **Клиент → API Gateway**
   ```
   POST http://localhost:8000/api/submit
   Content-Type: multipart/form-data
   Body: file=<файл>, sender=<ФИО>, work_id=<ID задания>
   ```

2. **API Gateway → File Storing Service**
   ```
   POST http://file_storing:8001/upload
   Content-Type: multipart/form-data
   Body: file=<файл>
   ```
   - Gateway извлекает файл из запроса клиента
   - Проксирует файл в File Storing Service
   - **Обработка ошибок**: если File Storing недоступен (502), Gateway возвращает клиенту ошибку 502

3. **File Storing Service → API Gateway**
   ```
   Response: 200 OK
   Body: {"filename": "work.txt"}
   ```
   - Файл сохранён в `/files` (Docker volume)
   - Возвращается имя сохранённого файла

4. **API Gateway → File Analysis Service**
   ```
   POST http://file_analysis:8002/analyze
   Content-Type: application/json
   Body: {
     "file_name": "work.txt",
     "sender": "Иванов Иван",
     "work_id": "hw1"
   }
   ```
   - Gateway формирует JSON-запрос с метаданными
   - **Обработка ошибок**: если Analysis Service недоступен (502), Gateway возвращает клиенту ошибку 502

5. **File Analysis Service → Qdrant**
   ```
   GET http://qdrant:6333/collections/documents
   ```
   - Проверка готовности Qdrant и существования коллекции
   - При первом запуске создаётся коллекция с векторами размерности 384

6. **File Analysis Service → Qdrant (сохранение вектора)**
   ```
   PUT http://qdrant:6333/collections/documents/points?wait=true
   Content-Type: application/json
   Body: {
     "points": [{
       "id": <числовой ID>,
       "vector": [0.1, 0.2, ...],  // 384-мерный вектор
       "payload": {
         "sender": "Иванов Иван",
         "work_id": "hw1",
         "file_name": "work.txt",
         "timestamp": "2025-12-12T..."
       }
     }]
   }
   ```
   - Файл читается из `/files`
   - Текст преобразуется в вектор через hash-функцию (демо-версия)
   - Вектор сохраняется в Qdrant с метаданными

7. **File Analysis Service → Qdrant (поиск похожих)**
   ```
   POST http://qdrant:6333/collections/documents/points/search
   Content-Type: application/json
   Body: {
     "vector": [0.1, 0.2, ...],
     "limit": 5,
     "score_threshold": 0.7,
     "filter": {
       "must": [{"key": "work_id", "match": {"value": "hw1"}}],
       "must_not": [{"key": "sender", "match": {"value": "Иванов Иван"}}]
     }
   }
   ```
   - Поиск похожих работ того же задания от других студентов
   - Возвращается список с оценками схожести (cosine similarity)

8. **File Analysis Service → API Gateway**
   ```
   Response: 200 OK
   Body: {"similarity": 0.95}
   ```
   - Если найдены похожие работы, возвращается максимальная схожесть
   - Если похожих нет, возвращается 0.0
   - Отчёт сохраняется в `/files/reports/`

9. **API Gateway → Клиент**
   ```
   Response: 200 OK
   Body: {"similarity": 0.95}
   ```

**Обработка ошибок:**
- Если File Storing недоступен: Gateway → 502 Bad Gateway
- Если File Analysis недоступен: Gateway → 502 Bad Gateway
- Если Qdrant недоступен: File Analysis логирует ошибку, возвращает 500
- Если файл не найден: File Analysis → 404 Not Found

---

### Сценарий 2: Получение отчётов преподавателем

**Пользовательский сценарий:** Преподаватель запрашивает все отчёты по конкретному заданию.

**Технический сценарий обмена данными:**

1. **Клиент → API Gateway**
   ```
   GET http://localhost:8000/api/works/hw1/reports
   ```

2. **API Gateway → File Analysis Service**
   ```
   GET http://file_analysis:8002/reports/hw1
   ```
   - Gateway извлекает work_id из URL и проксирует запрос

3. **File Analysis Service → Файловая система**
   - Читает все JSON-файлы из `/files/reports/`
   - Фильтрует отчёты по work_id
   - Парсит JSON-отчёты

4. **File Analysis Service → API Gateway**
   ```
   Response: 200 OK
   Body: [
     {
       "file_name": "work1.txt",
       "sender": "Иванов Иван",
       "work_id": "hw1",
       "plagiarized": false,
       "similarity": 0.0,
       "timestamp": "2025-12-12T..."
     },
     {
       "file_name": "work2.txt",
       "sender": "Петров Петр",
       "work_id": "hw1",
       "plagiarized": true,
       "similarity": 0.95,
       "timestamp": "2025-12-12T..."
     }
   ]
   ```

5. **API Gateway → Клиент**
   ```
   Response: 200 OK
   Body: [<массив отчётов>]
   ```

**Обработка ошибок:**
- Если отчётов не найдено: File Analysis → 404 Not Found
- Если File Analysis недоступен: Gateway → 502 Bad Gateway

---

## Алгоритм проверки на плагиат

1. **Преобразование текста в вектор:**
   - Текст файла хешируется через SHA-1
   - Хеш преобразуется в 384-мерный нормализованный вектор
   - Вектор имеет единичную длину (для корректного cosine similarity)

2. **Сохранение в векторную БД:**
   - Вектор сохраняется в Qdrant с метаданными (sender, work_id, file_name)
   - Используется косинусная метрика расстояния

3. **Поиск похожих работ:**
   - Поиск выполняется только среди работ того же задания (work_id)
   - Исключаются работы того же студента (sender)
   - Порог схожести: 0.7 (score_threshold)
   - Возвращается до 5 наиболее похожих работ

4. **Определение плагиата:**
   - Если максимальная схожесть > 0.95 → работа помечается как плагиат
   - Значение схожести сохраняется в отчёте

---

## Быстрый старт

```bash
git clone <repo_url>
cd software-design-hw3
docker compose up --build
```

После запуска:
- API Gateway: http://localhost:8000
- Примеры запросов: postman_collection.json

---

## Демонстрационный тестовый скрипт

```bash
brew install jq     # или sudo apt install jq
bash demo_scripts.sh
```

Скрипт создаёт три тестовые работы (две идентичные, одна уникальная), отправляет их в систему и выводит результаты анализа.

---

## API Endpoints

### Через API Gateway (localhost:8000)

- **POST** `/api/submit`
  - Загрузка работы на проверку
  - Параметры: `file` (multipart), `sender` (form), `work_id` (form)
  - Ответ: `{"similarity": <float>}`

- **GET** `/api/works/{work_id}/reports`
  - Получение всех отчётов по заданию
  - Ответ: массив JSON-объектов с отчётами

### Прямые endpoints микросервисов

- **File Storing**: `POST /upload`, `GET /files/{filename}`
- **File Analysis**: `POST /analyze`, `GET /reports/{work_id}`, `GET /health`

---

## Структура проекта

```
software-design-hw3/
├── gateway/              # API Gateway сервис
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── file_storing/         # Сервис хранения файлов
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── file_analysis/        # Сервис анализа и проверки
│   ├── main.go
│   ├── Dockerfile
│   └── go.mod
├── docker-compose.yml    # Конфигурация всех сервисов
├── postman_collection.json
├── demo_scripts.sh
├── README.md
└── instruction.md
```

---

**Автор:** Качаев Дмитрий, 2025
