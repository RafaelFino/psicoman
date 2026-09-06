# Psicoman — Makefile
# Alvos: build (dois binários), test (unit), e2e, lint, migrate, run.

GO        ?= go
BIN_DIR   ?= bin
ADMIN_BIN := $(BIN_DIR)/psicoman-admin
PORTAL_BIN:= $(BIN_DIR)/psicoman-portal

.PHONY: all build build-admin build-portal test e2e lint tidy clean run-admin run-portal

all: build

## build: compila os dois binários (saída = arquivo, não diretório)
build: build-admin build-portal

build-admin:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(ADMIN_BIN) ./cmd/admin

build-portal:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(PORTAL_BIN) ./cmd/portal

## test: testes unitários (exclui a suíte E2E)
test:
	$(GO) test ./internal/... ./cmd/...

## e2e: testes end-to-end
e2e:
	$(GO) test ./test/e2e/...

## lint: vet + verificação de formatação
lint:
	$(GO) vet ./...
	@test -z "$$(gofmt -l . 2>/dev/null)" || (echo "arquivos não formatados:"; gofmt -l .; exit 1)

## tidy: organiza go.mod/go.sum
tidy:
	$(GO) mod tidy

## migrate: aplica migrations (as migrations rodam no boot; alvo p/ CI)
migrate: build-admin
	@echo "As migrations são aplicadas automaticamente no boot do binário."

## run-admin / run-portal: executa localmente com config.yaml
run-admin: build-admin
	$(ADMIN_BIN) -config config.yaml

run-portal: build-portal
	$(PORTAL_BIN) -config config.yaml

## clean: remove binários
clean:
	rm -rf $(BIN_DIR)
