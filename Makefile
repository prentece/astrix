.PHONY: help build test test-fast clean tidy

default: help

help:
	@echo "Astrix - Development Tasks:"
	@echo "  make build      - Compila o binário em bin/astrix"
	@echo "  make test       - Executa todos os testes unitários"
	@echo "  make test-fast  - Executa testes sem cache com concorrência"
	@echo "  make tidy       - Executa go mod tidy"
	@echo "  make clean      - Remove binários temporários"

VERSION ?= 0.1.0
LDFLAGS := -s -w -X 'astrix/internal/cli.Version=$(VERSION)'

build:
	@mkdir -p bin
	go build -tags "sqlite_foreign_keys" -ldflags "$(LDFLAGS)" -o bin/astrix ./cmd/astrix

test:
	go test -tags "sqlite_foreign_keys" -v ./...

test-fast:
	go test -tags "sqlite_foreign_keys" ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/

npm-prepare-local: build
	@mkdir -p npm/platforms/linux-x64/bin
	@cp bin/astrix npm/platforms/linux-x64/bin/astrix
	@chmod +x npm/platforms/linux-x64/bin/astrix npm/astrix/bin/astrix.js
	@echo "Pacote local preparado para linux-x64."

npm-test: npm-prepare-local
	@node npm/astrix/bin/astrix.js version
	@node npm/astrix/bin/astrix.js --help
