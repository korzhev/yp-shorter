# Сокращатель ссылок

Пример проекта

## Локальный запуск

- `go mod tidy` - резолв зафисимостей(не работает при впн)
- `export PATH=$PATH:$(go env GOPATH)/bin` - прокинуть в консоль mockgen
- `mockgen -source=internal/config/db/interfaces.go -destination=mocks/mock_db.go -package=mocks IPG` - пример генерации моков для тестов
- `set -a && source .env && set +a` - загрузить переменные окружения в терминал
- `sudo docker compose up -d` - В режиме демона запуск Postgres
- `go test ./internal/... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html` - отчёт по покрытию тестами
- `go test -count=1 ./...` - прогон тестов
- `go test ./cmd/shortener -run '^$' -bench '^BenchmarkHTTPRoutes$' -benchtime=1x -count=1 -benchmem -memprofile=profiles/base.pprof` - прогон бенчмарка с профилировщиком

## HTTPS

По умолчанию сервер работает по HTTP. Флаг `-s` или переменная окружения
`ENABLE_HTTPS=true` включает HTTPS через `tls.Listen` (минимальная версия TLS — 1.2).
Переменные окружения имеют приоритет над флагами: `ENABLE_HTTPS=false`
отключает HTTPS даже при наличии `-s`.

Сертификат и приватный ключ в формате PEM загружаются из `cert.pem` и `key.pem`
относительно рабочего каталога. Пути можно изменить флагами `-tls-cert`,
`-tls-key` либо переменными `TLS_CERT_FILE`, `TLS_KEY_FILE`.
При отсутствии или некорректности файлов сервер сообщает об ошибке и не
запускается; автоматического перехода на HTTP нет. В HTTP-режиме файлы не нужны.

Пример запуска в PowerShell после подготовки сертификата и ключа:

```powershell
Set-Location C:\goprojects\yp-shorter
go run ./cmd/shortener -s -b https://localhost:8080
# Альтернатива:
$env:ENABLE_HTTPS = 'true'
go run ./cmd/shortener -b https://localhost:8080
```

Базовый URL коротких ссылок задаётся отдельно через `-b` или `BASE_URL`:
включение HTTPS не меняет его автоматически. Самоподписанные сертификаты
используйте только для разработки, настроив доверие клиента к сертификату.
Не добавляйте приватные ключи в Git. Сервер профилировщика остаётся на HTTP
по адресу `127.0.0.1:6060`.

## Метаданные сборки

Версия, дата и коммит выводятся при запуске. Без параметров сборки каждое
значение равно `N/A`. Передать их можно через `-ldflags` и флаг `-X`:

```sh
go build -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=2026-10-27T12:00:00Z -X main.buildCommit=bd711e9" -o shortener ./cmd/shortener
```

Замените значения на версию релиза, дату сборки и хеш коммита вашего исходного
кода (`git rev-parse --short HEAD`). Каждый флаг `-X` задаёт одну строковую
переменную пакета `main`.

## Результат сравнения двух профилей памяти

```text
File: main
Build ID: 07f1f2d4cce5c794c7751d219ba367c1515fa805
Type: inuse_space
Time: 2026-10-06 23:32:35 MSK
Showing nodes accounting for -1964.99kB, 38.90% of 5051.11kB total
      flat  flat%   sum%        cum   cum%
 -902.59kB 17.87% 17.87% -1451.43kB 28.73%  compress/flate.NewWriter (inline)
 -548.84kB 10.87% 28.73%  -548.84kB 10.87%  compress/flate.(*compressor).initDeflate (inline)
 -513.56kB 10.17% 38.90%  -513.56kB 10.17%  sync.(*Pool).pinSlow
         0     0% 38.90%  -548.84kB 10.87%  compress/flate.(*compressor).init
         0     0% 38.90% -1451.43kB 28.73%  compress/gzip.(*Writer).Write
         0     0% 38.90% -1964.99kB 38.90%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 38.90% -1451.43kB 28.73%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 38.90% -1451.43kB 28.73%  github.com/go-chi/chi/v5/middleware.Recoverer.func1
         0     0% 38.90% -1451.43kB 28.73%  github.com/go-chi/chi/v5/middleware.RedirectSlashes.func1
         0     0% 38.90% -1451.43kB 28.73%  github.com/korzhev/yp-shorter/internal/handler.ShortLinkHandler.APIGetLinksByUserIDHandlerFunc
         0     0% 38.90% -1451.43kB 28.73%  github.com/korzhev/yp-shorter/internal/middleware.(*compressWriter).Write
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap.(*Logger).With
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap.(*SugaredLogger).With
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/buffer.Pool.Get
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/internal/pool.(*Pool[go.shape.*uint8]).Get (inline)
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/zapcore.(*ioCore).With
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/zapcore.(*ioCore).clone (inline)
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/zapcore.(*jsonEncoder).Clone
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/zapcore.(*jsonEncoder).clone
         0     0% 38.90%  -513.56kB 10.17%  go.uber.org/zap/zapcore.(*sampler).With
         0     0% 38.90% -1451.43kB 28.73%  main.RootRouter.NewAuthMiddleware.func2.1
         0     0% 38.90% -1451.43kB 28.73%  main.RootRouter.NewCompressorMiddleware.func3.1
         0     0% 38.90% -1964.99kB 38.90%  main.RootRouter.NewLoggerMiddleware.func1.1
         0     0% 38.90%  -513.56kB 10.17%  main.RootRouter.NewLoggerMiddleware.func1.1.1
         0     0% 38.90% -1964.99kB 38.90%  net/http.(*conn).serve
         0     0% 38.90% -1964.99kB 38.90%  net/http.HandlerFunc.ServeHTTP
         0     0% 38.90% -1964.99kB 38.90%  net/http.serverHandler.ServeHTTP
         0     0% 38.90%  -513.56kB 10.17%  sync.(*Pool).Get
         0     0% 38.90%  -513.56kB 10.17%  sync.(*Pool).pin
```
