.PHONY: help build test test-fast clean tidy

default: help

help:
	@echo "Astrix - Development Tasks:"
	@echo "  make build      - Compila o binário em bin/astrix"
	@echo "  make test       - Executa todos os testes unitários"
	@echo "  make test-fast  - Executa testes sem cache com concorrência"
	@echo "  make tidy       - Executa go mod tidy"
	@echo "  make clean      - Remove binários temporários"

build:
	@mkdir -p bin
	go build -tags "sqlite_foreign_keys" -ldflags "-s -w" -o bin/astrix ./cmd/astrix

test:
	go test -tags "sqlite_foreign_keys" -v ./...

test-fast:
	go test -tags "sqlite_foreign_keys" ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
