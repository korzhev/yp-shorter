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
