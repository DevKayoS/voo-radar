.PHONY: run build tidy fmt test

# Roda a coleta localmente, carregando credenciais do .env (se existir).
run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/radar

build:
	@go build -o bin/radar ./cmd/radar

tidy:
	@go mod tidy

fmt:
	@gofmt -w .

test:
	@go test ./...
