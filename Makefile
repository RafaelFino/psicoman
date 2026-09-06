# Psicoman — Makefile
# Qualidade: make check (fmt + vet + lint + testes + e2e).

GO         ?= go
BIN_DIR    ?= bin
ADMIN_BIN  := $(BIN_DIR)/psicoman-admin
PORTAL_BIN := $(BIN_DIR)/psicoman-portal
CONFIG     ?= config.yaml
DEV_CONFIG ?= config.dev.yaml

.PHONY: help all build build-admin build-portal test test-unit e2e \
        fmt fmt-check vet lint check tidy migrate \
        run-admin run-portal run-local deploy update clean

## help: lista os alvos disponíveis
help:
	@echo "Psicoman — alvos disponíveis:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

all: build

## build: compila os dois binários (saída = arquivo, não diretório)
build: build-admin build-portal

build-admin:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(ADMIN_BIN) ./cmd/admin

build-portal:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(PORTAL_BIN) ./cmd/portal

## test: TODOS os testes (unitários + E2E)
test: test-unit e2e

## test-unit: testes unitários (exclui a suíte E2E)
test-unit:
	$(GO) test ./internal/... ./cmd/...

## e2e: testes end-to-end
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

## migrate: as migrations rodam no boot; alvo informativo p/ CI
migrate: build-admin
	@echo "As migrations são aplicadas automaticamente no boot do binário."

## run-admin: executa o admin com $(CONFIG)
run-admin: build-admin
	$(ADMIN_BIN) -config $(CONFIG)

## run-portal: executa o portal com $(CONFIG)
run-portal: build-portal
	$(PORTAL_BIN) -config $(CONFIG)

## run-local: sobe admin+portal em MODO DEV (auth desligada) para validar local
run-local: build $(DEV_CONFIG)
	@echo "Subindo Psicoman em MODO DEV (autenticação DESLIGADA). Ctrl-C para parar."
	@echo "Admin:  http://localhost:8080   Portal: http://localhost:8081"
	@trap 'kill 0' INT TERM; \
	 $(ADMIN_BIN)  -config $(DEV_CONFIG) & \
	 $(PORTAL_BIN) -config $(DEV_CONFIG) & \
	 wait

# Gera um config de desenvolvimento local se ainda não existir.
$(DEV_CONFIG):
	@echo "Gerando $(DEV_CONFIG) (modo dev, dados em ./data-dev) ..."
	@printf '%s\n' \
	  '# Config de DESENVOLVIMENTO LOCAL — auth desligada. Não use em produção.' \
	  'dev_mode: true' \
	  'admin:' \
	  '  host: "0.0.0.0"' \
	  '  port: 8080' \
	  'portal:' \
	  '  host: "0.0.0.0"' \
	  '  port: 8081' \
	  'paths:' \
	  '  sqlite: "./data-dev/psicoman.db"' \
	  '  ged_root: "./data-dev/ged"' \
	  '  log_dir: "./data-dev/logs"' \
	  'admin_auth:' \
	  '  email: "dev@local"' \
	  'crypto:' \
	  "  key: \"$$(openssl rand -base64 32 2>/dev/null || echo MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=)\"" \
	  'log:' \
	  '  level: "debug"' \
	  > $(DEV_CONFIG)

## deploy: instalação interativa (ver docs/deploy.md)
deploy:
	sudo ./scripts/deploy.sh

## update: atualiza a instância a partir do GitHub (backup + validação + rollback)
update:
	sudo ./scripts/update-server.sh

## clean: remove binários e o config de dev
clean:
	rm -rf $(BIN_DIR) $(DEV_CONFIG)
