# Movie Service (ДЗ 3)

Go-сервис с HTTP и gRPC API над внешним фильм-стендом.

## Запуск

```powershell
go run .
```

Обязательных аргументов командной строки нет.

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `MOVIE_BASE_URL` | `https://homeworksite.site` | базовый URL стенда |
| `HTTP_ADDR` | `:8080` | адрес HTTP-сервера |
| `GRPC_ADDR` | `:9090` | адрес gRPC-сервера |

Фильм запрашивается по адресу `{MOVIE_BASE_URL}/{id}/info.0.json`.

Swagger: <http://localhost:8080/swagger/index.html>.

## HTTP API

`GET /load-movies?from=N&to=M` — фильмы в диапазоне `[from; to]`.

Успех — `200` и JSON-массив:

```json
[{"id":1,"title":"Solaris","year":1972,"director":"Andrei Tarkovsky"}]
```

Если часть фильмов не загрузилась — `207` и заголовок `X-Failed-Ids`:

```text
X-Failed-Ids: 4,5
```

Ошибки:

- `400` — неверные `from`/`to`;
- `404` — в диапазоне нет ни одного фильма;
- `502` — ошибка стенда;
- `504` — таймаут стенда.

`GET /last-movie` — последний фильм на стенде.

## gRPC API

```proto
service MovieService {
  rpc LoadMovies(LoadMoviesRequest) returns (LoadMoviesResponse);
  rpc GetLastMovie(GetLastMovieRequest) returns (Movie);
}
```

Контракт: [`api/movie/v1/movie.proto`](api/movie/v1/movie.proto).

Некорректные аргументы возвращают `InvalidArgument`, пустой стенд — `NotFound`,
ошибки стенда — `Unavailable` или `DeadlineExceeded`.

```bash
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -d '{"from":1,"to":3}' \
  localhost:9090 movie.v1.MovieService/LoadMovies
grpcurl -plaintext -d '{}' \
  localhost:9090 movie.v1.MovieService/GetLastMovie
```

## Структура

```text
main.go                  — конфиг, сборка зависимостей, запуск серверов
internal/movie/          — модель, ошибки, бизнес-логика, worker pool
internal/stand/          — клиент внешнего стенда
internal/api/            — HTTP- и gRPC-хендлеры
api/movie/v1/            — protobuf-контракт
docs/                    — Swagger
```

`internal` — стандартная папка Go для приватного кода модуля. Её пакеты нельзя
импортировать из другого проекта.

## Проверки

```bash
go test ./...
go vet ./...
```

Автотесты курса:

```powershell
git clone --depth 1 https://github.com/vacmannnn/go-courses "$env:TEMP\go-courses-tests"
cd "$env:TEMP\go-courses-tests\autotests"
python -m pip install -r requirements.txt
python scripts/ci.py `
  --repo C:\university\golang\go-spbu-practice `
  --pr-title "ДЗ 3"
```

На Windows `check.py` собирает бинарник без `.exe`, поэтому локально тесты
удобнее запустить вручную:

```powershell
cd C:\university\golang\go-spbu-practice
go build -o "$env:TEMP\movie-service.exe" .

cd "$env:TEMP\go-courses-tests\autotests"
$env:COURSE_SOLUTION_BIN = "$env:TEMP\movie-service.exe"
go test -race -count=1 -timeout=120s -run '^TestCourse' ./task3
```

Для сдачи откройте PR в `main` с заголовком `ДЗ 3 — HTTP/gRPC API`.
Файл `.github/workflows/course.yml` уже подключён.
