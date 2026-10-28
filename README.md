# Movie Service — HTTP + gRPC API (ДЗ 3)

Go-сервис, который предоставляет HTTP и gRPC API над внешним фильм-стендом.
Это простая версия для ДЗ 3: код разделён на небольшие пакеты, но без лишних
архитектурных слоёв.

## Что умеет

- `GET /load-movies?from=N&to=M` — загружает фильмы в диапазоне `[from; to]`.
- `GET /last-movie` — находит последний доступный фильм на стенде.
- gRPC `MovieService/LoadMovies` и `MovieService/GetLastMovie`.
- Swagger UI: `http://localhost:8080/swagger/index.html`.
- Каждый запрос к стенду идёт через `context` с таймаутом 5 секунд.
- Ошибки стенда превращаются в понятные HTTP-статусы и gRPC-коды.
- Частичные ошибки диапазона видны в ответе.

## Запуск

Нужен Go 1.25+. Обязательных CLI-аргументов нет.

```powershell
go run .
```

По умолчанию:

- стенд: `https://homeworksite.site`;
- HTTP: `:8080`;
- gRPC: `:9090`.

Проверка:

```powershell
curl.exe "http://localhost:8080/load-movies?from=1&to=3"
curl.exe "http://localhost:8080/last-movie"
```

Swagger открывается в браузере: <http://localhost:8080/swagger/index.html>.

### Локальный стенд

Если внешний стенд недоступен, можно поднять маленькую заглушку. Она лежит в
`testdata`, поэтому не попадает в `go build ./...` и `go test ./...`.

```powershell
# терминал 1
$env:MOCK_STAND_LAST_ID = "10"
go run ./testdata/mockstand

# терминал 2
$env:MOVIE_BASE_URL = "http://localhost:18080"
go run .
```

Bash:

```bash
MOCK_STAND_LAST_ID=10 go run ./testdata/mockstand
MOVIE_BASE_URL=http://localhost:18080 go run .
```

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `MOVIE_BASE_URL` | `https://homeworksite.site` | Базовый URL стенда. Фильм запрашивается по `{MOVIE_BASE_URL}/{id}/info.0.json`. |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера. |
| `GRPC_ADDR` | `:9090` | Адрес gRPC-сервера. |

Пустая переменная означает «взять значение по умолчанию».

## HTTP API

### `GET /load-movies?from=N&to=M`

Успех — `200` и JSON-массив:

```json
[
  {
    "id": 1,
    "title": "Solaris",
    "year": 1972,
    "director": "Andrei Tarkovsky",
    "description": "A psychologist visits a space station",
    "actors": ["Donatas Banionis"],
    "genres": ["science fiction"]
  }
]
```

Если часть фильмов не загрузилась, возвращается `207 Multi-Status`: в теле —
успешно загруженные фильмы, в заголовке `X-Failed-Ids` — id через запятую.

Ошибки:

- `400` — `from`/`to` отсутствуют, не числа, меньше или равны нулю, либо `from > to`;
- `404` — в диапазоне нет ни одного фильма;
- `502` — стенд вернул ошибку или недоступен;
- `504` — стенд не ответил за таймаут.

### `GET /last-movie`

Успех — `200` и JSON одного фильма. Если стенд пуст — `404`.

По условию задачи id на стенде идут подряд от `1` до `last_id`, а `99999` —
верхняя граница. Сначала проверяется id `99999`. Если его нет, бинарным поиском
находится последний существующий id.

## gRPC API

Контракт: [`api/movie/v1/movie.proto`](api/movie/v1/movie.proto).

```proto
service MovieService {
  rpc LoadMovies(LoadMoviesRequest) returns (LoadMoviesResponse);
  rpc GetLastMovie(GetLastMovieRequest) returns (Movie);
}
```

`LoadMoviesResponse` содержит `movies` и `errors` — ошибки по отдельным фильмам.
Некорректные `from`/`to` дают `InvalidArgument`, пустой стенд — `NotFound`,
ошибки стенда — `Unavailable` или `DeadlineExceeded`.

Пример через `grpcurl` (включён gRPC reflection):

```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -d '{"from":1,"to":3}' \
  localhost:9090 movie.v1.MovieService/LoadMovies
grpcurl -plaintext -d '{}' \
  localhost:9090 movie.v1.MovieService/GetLastMovie
```

## Структура

```text
main.go                  — конфиг, сборка зависимостей, запуск HTTP и gRPC
internal/movie/          — Movie, ошибки и бизнес-логика
internal/stand/          — HTTP-клиент внешнего стенда
internal/api/            — HTTP- и gRPC-хендлеры
api/movie/v1/            — protobuf-контракт и сгенерированный код
docs/                    — сгенерированный Swagger
testdata/mockstand/      — локальная заглушка стенда
```

Зависимости простые:

```text
main -> api -> movie
main -> stand -> movie
```

`movie` не знает ни про HTTP, ни про gRPC, ни про конкретный сайт: он работает
с интерфейсом `MovieProvider`. Реальный клиент стенда подключается в `main`.

Диапазон загружается пулом из 8 воркеров: канал `jobs` раздаёт id, воркеры
параллельно ходят на стенд, результаты собираются и сортируются по id.
Количество воркеров задаётся константой `defaultWorkers` в
`internal/movie/movie.go`.

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

Сгенерированные файлы лежат в репозитории, поэтому для обычной сборки `protoc`
и `swag` не нужны.

## Автотесты курса

`grade.yml` — это переиспользуемый workflow курса. Он не запускается сам по себе
из корня репозитория. В свой репозиторий нужно один раз положить
`ci-example.yml` из репозитория курса как `.github/workflows/course.yml`.
Этот файл вызывает `vacmannnn/go-courses/.github/workflows/grade.yml@main`.

Дальше достаточно открыть PR в `main` с номером задания в заголовке:

```text
ДЗ 3 — HTTP/gRPC API
```

или `task3`. Результат появится во вкладке **Checks** PR. Если изменить заголовок
PR, проверка запустится заново. Параметры workflow менять не нужно.

### Локальный запуск автотестов

```powershell
# 1. Клонировать тесты курса
git clone --depth 1 https://github.com/vacmannnn/go-courses "$env:TEMP\go-courses-tests"

# 2. Поставить зависимость тестов
cd "$env:TEMP\go-courses-tests\autotests"
python -m pip install -r requirements.txt

# 3. Запустить проверку ДЗ 3 для этого репозитория
python scripts/ci.py `
  --repo C:\university\golang\go-spbu-practice `
  --pr-title "ДЗ 3" `
  --report "$env:TEMP\task3-report.json"
```

`ci.py` сам найдёт `go.mod` в корне и единственный `main`-пакет — корень
репозитория. После этого он соберёт сервис и запустит 5 HTTP-тестов из
`task3/http_test.go`.

На Linux и в GitHub Actions этого достаточно. На Windows есть особенность
самого тестового скрипта: `check.py` собирает бинарник с именем `student-app`
без `.exe`, а Windows не находит такой файл при запуске. Поэтому локально на
Windows тесты удобнее запустить вручную с готовым `.exe`:

```powershell
cd C:\university\golang\go-spbu-practice
go build -o "$env:TEMP\movie-service.exe" .

cd "$env:TEMP\go-courses-tests\autotests"
$env:COURSE_SOLUTION_BIN = "$env:TEMP\movie-service.exe"
go test -race -count=1 -timeout=120s -run '^TestCourse' ./task3
```

Ожидаемый результат: `ok go-course-tests/task3`.

## Ветки

- `movie-service` — простая версия сервиса для ДЗ 3.
- Для ДЗ 4 можно ответвить отдельную ветку и при необходимости разнести
  `movie` на `domain` и `usecase`, а `api` — на `handler`.
- Предыдущие ДЗ остаются в своих ветках.
