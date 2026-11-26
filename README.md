# Movie Service

HTTP/gRPC сервис фильмов. PostgreSQL — основное хранилище. Redis — обратный индекс и кэш поисковых ответов. Внешний фильм-стенд используется только для синхронизации при старте.

## Запуск

### PostgreSQL

```bash
docker run --rm -d --name movies-postgres \
  -e POSTGRES_DB=movies \
  -e POSTGRES_USER=course \
  -e POSTGRES_PASSWORD=course \
  -p 5432:5432 \
  postgres:16-alpine
```

### Redis

```bash
docker run --rm -d --name movies-redis -p 6379:6379 redis:7-alpine
```

### Приложение

Linux/macOS:

```bash
export DATABASE_URL="postgres://course:course@localhost:5432/movies?sslmode=disable"
export REDIS_ADDR="localhost:6379"
go run ./cmd/server
```

Windows PowerShell:

```powershell
$env:DATABASE_URL = "postgres://course:course@localhost:5432/movies?sslmode=disable"
$env:REDIS_ADDR = "localhost:6379"
go run ./cmd/server
```

Миграции применяются автоматически при старте из `internal/adapter/postgres/migrations`.

Порядок старта: миграции → синхронизация фильмов → построение поискового индекса → HTTP/gRPC.

## Переменные окружения

| Переменная | Обязательна | По умолчанию |
|---|---|---|
| `MOVIE_BASE_URL` | нет | `https://homeworksite.site` |
| `HTTP_ADDR` | нет | `:8080` |
| `DATABASE_URL` | да | — |
| `REDIS_ADDR` | да | — |
| `GRPC_ADDR` | нет | `:9090` |

## Индекс

Ключ:

```text
search:index:<слово>
```

Тип: Redis SET с id фильмов. TTL нет.

Слова берутся из `title` и `director`, приводятся к нижнему регистру и разбиваются регулярным выражением `[\p{L}\p{N}]+`.

Индекс полностью перестраивается при каждом старте:

1. удаляются старые ключи `search:index:*`;
2. удаляется кэш `search:cache:*`;
3. все фильмы читаются из PostgreSQL;
4. индекс записывается заново.

## Поиск

```http
GET /search?q=...
```

- `q` обрезается по краям;
- пустой `q` → `400`;
- id ищутся в Redis, фильмы читаются из PostgreSQL по этим id;
- нет совпадений → `200` и `[]`;
- поиск регистронезависимый, по целым словам, без морфологии и подстроки.

Для нескольких слов используется **AND**: через `SINTER` остаются фильмы, содержащие все слова.

Пример: `q=Wes Anderson` найдёт фильм, у которого есть и `Wes`, и `Anderson`.

## Кэш

Ключ:

```text
search:cache:<q после trim>
```

Значение: полный JSON-ответ. TTL — `3600` секунд.

При чтении из кэша TTL не продлевается. Кэш сбрасывается при перестройке индекса.

## Примеры

```bash
curl "http://localhost:8080/search?q=Solaris"
curl "http://localhost:8080/search?q=Andrei"
curl "http://localhost:8080/search?q=Tarkovsky"
curl "http://localhost:8080/search?q=SOLARIS"
curl "http://localhost:8080/search?q=Wes%20Anderson"
curl "http://localhost:8080/search?q=unknown"
```
