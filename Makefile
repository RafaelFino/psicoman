# Psicoman — Makefile
# Binário único (cmd/psicoman) com o PocketBase embarcado como framework.
# Gate de qualidade: make check (fmt + vet + build + test-unit + e2e + lint).
# O lint NÃO é mais advisory: golangci-lint v2 é obrigatório e sua falha derruba
# o check. Instale/atualize com `make tools`. Escape explícito: `make check SKIP_LINT=1`.

GO         ?= go
BIN_DIR    ?= bin
BIN        := $(BIN_DIR)/psicoman
DATA_DIR   ?= ./pb_data
HTTP_ADDR  ?= 127.0.0.1:8090

# golangci-lint v2: versão fixada. Compilar com o toolchain local é obrigatório
# (binários pré-compilados não decodificam o export data do Go 1.27).
GOLANGCI_LINT_VERSION ?= v2.15.0

.PHONY: help all build test test-unit e2e \
        fmt fmt-check vet lint tools check tidy serve clean

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

## tools: instala o golangci-lint v2 fixado, compilado com o toolchain local
tools:
	@echo "==> instalando golangci-lint $(GOLANGCI_LINT_VERSION) (compilado com o Go local)"
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

## lint: golangci-lint v2 OBRIGATÓRIO (falha derruba o check). Pule só com SKIP_LINT=1.
# Fim do modo advisory: sem `|| echo`, sem `@true`, sem probe e sem `timeout` externo
# (o run.timeout do .golangci.yml governa). Três guardas antes de rodar:
#   1. golangci-lint ausente     -> falha pedindo `make tools`.
#   2. versão != a fixada        -> falha pedindo `make tools`.
#   3. ok                        -> roda `golangci-lint run ./...` de verdade.
lint: fmt-check vet
ifeq ($(SKIP_LINT),1)
	@echo "!! LINT PULADO POR PEDIDO EXPLÍCITO (SKIP_LINT=1)"
else
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
	  echo "!! golangci-lint não encontrado no PATH. Rode 'make tools' (instala $(GOLANGCI_LINT_VERSION))."; \
	  exit 1; \
	fi; \
	have="$$(golangci-lint version 2>/dev/null | grep -oE 'version [0-9]+\.[0-9]+\.[0-9]+' | awk '{print "v"$$2}')"; \
	want="$(GOLANGCI_LINT_VERSION)"; \
	if [ "$$have" != "$$want" ]; then \
	  echo "!! golangci-lint $$have difere da versão fixada $$want. Rode 'make tools'."; \
	  exit 1; \
	fi; \
	echo "==> golangci-lint $$have"; \
	golangci-lint run ./...
endif

## check: gate de qualidade (fmt + vet + build + testes + e2e + lint obrigatório)
check: fmt-check vet build test-unit e2e
	@$(MAKE) --no-print-directory lint SKIP_LINT=$(SKIP_LINT)
	@echo "==> check OK: formatado, vet limpo, build ok, testes verdes, lint limpo."

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
