# Movie Service — HTTP + gRPC API

Go-сервис, который предоставляет HTTP и gRPC API над внешним фильм-стендом.
Это базовая ветка `movie-service` для третьего и последующих домашних заданий:
сервис развивается в одном Go-модуле в корне репозитория.

## Возможности

- `GET /load-movies?from=N&to=M` — загрузка фильмов в диапазоне `[from; to]`.
- `GET /last-movie` — поиск последнего доступного фильма на стенде.
- `MovieService.LoadMovies` и `MovieService.GetLastMovie` по gRPC.
- Swagger UI: `http://localhost:8080/swagger/index.html`.
- Контекст и таймаут на каждый запрос к стенду.
- Частичные ошибки диапазона не теряются: HTTP возвращает `207 Multi-Status`
  с успешными фильмами и заголовками `X-Failed-Ids` / `X-Partial-Errors`,
  gRPC возвращает `errors` в `LoadMoviesResponse` и `partial = true`.

## Быстрый старт

Нужен Go 1.25+. Сервис запускается без обязательных CLI-аргументов.

```powershell
go run .
```

По умолчанию сервис ходит на `https://homeworksite.site`, HTTP слушает `:8080`,
gRPC — `:9090`. Swagger открывается по адресу
`http://localhost:8080/swagger/index.html`.

### Локальный mock-стенд

Если внешний стенд недоступен, для проверки можно поднять локальную заглушку.
В ней фильмы имеют id от `1` до `MOCK_STAND_LAST_ID`:

```powershell
# терминал 1
$env:MOCK_STAND_LAST_ID = "10"
go run ./cmd/mock-stand

# терминал 2
$env:MOVIE_BASE_URL = "http://localhost:18080"
go run .
```

Bash:

```bash
MOCK_STAND_LAST_ID=10 go run ./cmd/mock-stand
MOVIE_BASE_URL=http://localhost:18080 go run .
```

Примеры:

```bash
curl "http://localhost:8080/load-movies?from=1&to=3"
curl "http://localhost:8080/last-movie"
```

## Конфигурация

Конфигурация читается только из переменных окружения. Пустое значение означает
«использовать значение по умолчанию».

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `MOVIE_BASE_URL` | `https://homeworksite.site` | Базовый URL стенда. Фильм запрашивается по `{MOVIE_BASE_URL}/{id}/info.0.json`. |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера. |
| `GRPC_ADDR` | `:9090` | Адрес gRPC-сервера. |
| `MOVIE_REQUEST_TIMEOUT` | `5s` | Таймаут одного запроса к стенду. |
| `MOVIE_WORKERS` | `8` | Количество параллельных запросов при загрузке диапазона. |
| `SHUTDOWN_TIMEOUT` | `10s` | Таймаут graceful shutdown. |

`MOVIE_BASE_URL` и `HTTP_ADDR` — обязательные по заданию переменные; остальные
добавлены для удобства эксплуатации и тоже имеют значения по умолчанию.

## HTTP API

### `GET /load-movies?from=N&to=M`

Успешный ответ `200` — JSON-массив фильмов:

```json
[
  {"id": 1, "title": "Toy Story", "year": "1995", "director": "John Lasseter"}
]
```

Если часть фильмов не загрузилась, возвращается `207 Multi-Status`: в теле —
массив успешно загруженных фильмов, а ошибки — в заголовках:

- `X-Failed-Count` — количество ошибок;
- `X-Failed-Ids` — id через запятую (до 100 штук);
- `X-Partial-Errors` — URL-encoded JSON `[{"id": 3, "error": "..."}]` (до 100 штук);
- `X-Partial-Errors-Truncated` — `true`, если в заголовки попали только первые 100 ошибок.

Ошибки:

- `400` — отсутствующий/нечисловой/неположительный `from` или `to`, либо `from > to`
  (а также диапазон длиннее 100 000 id);
- `404` — в диапазоне не найдено ни одного фильма;
- `502` — стенд вернул ошибку или недоступен;
- `504` — таймаут стенда.

### `GET /last-movie`

Успешный ответ `200` — JSON одного фильма. Если стенд пуст — `404`.

Поиск последнего фильма использует условие «id идут подряд от 1 до `last_id`»:
сначала проверяется id `99999`; если он не найден, бинарным поиском ищется
последний существующий id в `[1; 99998]`.

## gRPC API

Контракт: [`api/movie/v1/movie.proto`](api/movie/v1/movie.proto).

```proto
service MovieService {
  rpc LoadMovies(LoadMoviesRequest) returns (LoadMoviesResponse);
  rpc GetLastMovie(GetLastMovieRequest) returns (Movie);
}
```

`LoadMoviesResponse` содержит `movies`, `errors` и `partial`. Некорректные
`from`/`to` возвращают `InvalidArgument`, пустой стенд — `NotFound`, ошибки
стенда — `Unavailable` / `DeadlineExceeded` / `Internal`.

Пример через `grpcurl` (gRPC reflection включён):

```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -import-path . -proto api/movie/v1/movie.proto \
  localhost:9090 movie.v1.MovieService/LoadMovies -d '{"from":1,"to":3}'
grpcurl -plaintext -import-path . -proto api/movie/v1/movie.proto \
  localhost:9090 movie.v1.MovieService/GetLastMovie -d '{}'
```

## Структура

```text
main.go                      — запуск HTTP и gRPC серверов, graceful shutdown
cmd/mock-stand/              — локальная заглушка стенда для разработки
internal/config/             — конфигурация из env
internal/movie/              — доменная модель и ошибки
internal/stand/              — HTTP-клиент внешнего стенда
internal/service/            — бизнес-логика: диапазон, частичные ошибки, поиск последнего
internal/transport/httpapi/  — HTTP-хендлеры, JSON-ошибки, middleware
internal/transport/grpcapi/  — gRPC-сервер, маппинг ошибок, интерсепторы
api/movie/v1/                — protobuf-контракт и сгенерированный код
docs/                        — сгенерированный Swagger (Swagger 2.0)
```

Зависимости слоёв: transport → service → stand → внешний стенд. Транспорты не
ходят в стенд напрямую и не дублируют бизнес-логику.

## Разработка

```bash
go test ./...
go vet ./...
```

Перегенерация protobuf:

```bash
protoc \
  --go_out=. --go_opt=module=github.com/khusainovilas/go-spbu-practice \
  --go-grpc_out=. --go-grpc_opt=module=github.com/khusainovilas/go-spbu-practice \
  api/movie/v1/movie.proto
```

Перегенерация Swagger (нужен `swag`):

```bash
swag init -g main.go -o docs --parseInternal
```

Сгенерированные файлы лежат в репозитории, поэтому для сборки и запуска
`protoc` и `swag` не обязательны.

## Ветки

- `movie-service` — базовая ветка этого сервиса.
- Следующее ДЗ: `git checkout -b hw4-... movie-service`.
- Предыдущие ДЗ (`HW1`, `HW2`) остаются в своих ветках и не смешиваются с сервисом.
