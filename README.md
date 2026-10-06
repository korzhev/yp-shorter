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

## Результат сравнения двух профилей памяти
```
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
