# Psicoman — Makefile
# Binário único (cmd/psicoman) com o PocketBase embarcado como framework.
# Gate de qualidade: make check (fmt + vet + build + test-unit + e2e; lint advisory).

GO         ?= go
BIN_DIR    ?= bin
BIN        := $(BIN_DIR)/psicoman
DATA_DIR   ?= ./pb_data
HTTP_ADDR  ?= 127.0.0.1:8090

.PHONY: help all build test test-unit e2e \
        fmt fmt-check vet lint check tidy serve clean

## help: lista os alvos disponíveis
help:
	@echo "Psicoman — alvos disponíveis:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

all: build

## build: compila o binário único (saída = ARQUIVO, nunca diretório)
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/psicoman

## test: TODOS os testes (unitários + E2E)
test: test-unit e2e

## test-unit: testes unitários (exclui a suíte E2E)
test-unit:
	$(GO) test ./internal/...

## e2e: testes end-to-end sobre o binário único
e2e:
	$(GO) test ./test/e2e/...

## fmt: formata todo o código (gofmt -w)
fmt:
	gofmt -w .

## fmt-check: falha se algum arquivo não estiver formatado
fmt-check:
	@unformatted="$$(gofmt -l . 2>/dev/null)"; \
	if [ -n "$$unformatted" ]; then \
	  echo "Arquivos não formatados (rode 'make fmt'):"; echo "$$unformatted"; exit 1; \
	fi

## vet: análise estática do go vet
vet:
	$(GO) vet ./...

## lint: golangci-lint se instalado (advisory); gate obrigatório é fmt+vet
lint: fmt-check vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  echo "==> golangci-lint (advisory)"; \
	  golangci-lint run ./... || echo "!! golangci-lint reportou avisos (não bloqueia o check)."; \
	else \
	  echo "==> golangci-lint não instalado; gate = gofmt + go vet."; \
	  echo "    Instale (opcional): https://golangci-lint.run/usage/install/"; \
	fi

## check: gate de qualidade (fmt + vet + build + testes + e2e); lint é advisory
check: fmt-check vet build test-unit e2e
	@$(MAKE) --no-print-directory lint
	@echo "==> check OK: formatado, vet limpo, build ok, testes verdes."

## tidy: organiza go.mod/go.sum
tidy:
	$(GO) mod tidy

## serve: sobe o binário único localmente (PocketBase + rotas /v1 + admin UI em /_/)
serve: build
	@echo "Subindo Psicoman em $(HTTP_ADDR) — admin UI em http://$(HTTP_ADDR)/_/  (Ctrl-C para parar)."
	$(BIN) serve --http $(HTTP_ADDR) --dir $(DATA_DIR)

## clean: remove binários e o data dir local
clean:
	rm -rf $(BIN_DIR) $(DATA_DIR)
