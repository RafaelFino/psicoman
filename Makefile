.PHONY: build run run-api seed-and-run test docker clean

# Build: único binário Go com templates e assets embutidos
build:
	go build -o bin/psicoman ./cmd/server

# ─────────────────────────────────────────────────────────────────
# make run → sobe o ambiente dev completo via Docker Compose.
# Inclui DEV_MODE=true, volumes locais mapeados.
# Acesse: http://localhost:8080
# ─────────────────────────────────────────────────────────────────
run:
	mkdir -p data/db data/ged data/logs
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

# Sobe em background (detached)
run-bg:
	mkdir -p data/db data/ged data/logs
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d

# Para o compose dev
stop:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

# API Go diretamente (sem compose), com DEV_MODE via .env.dev
run-api: build
	@echo ">>> DEV MODE — rotas /api/dev/* ativas"
	env $$(grep -v '^#' .env.dev | grep -v '^$$' | xargs) ./bin/psicoman

# ─────────────────────────────────────────────────────────────────
# make seed-and-run → limpa banco, sobe servidor e carrega dados de teste.
# Idempotente: pode rodar várias vezes sem duplicar dados.
# ─────────────────────────────────────────────────────────────────
seed-and-run: build
	@rm -f data/db/dev.sqlite*
	@mkdir -p data/db data/ged data/logs
	@echo ">>> Iniciando servidor em background..."
	@env $$(grep -v '^#' .env.dev | grep -v '^$$' | xargs) ./bin/psicoman & \
		SERVER_PID=$$!; \
		sleep 2; \
		echo ">>> Carregando dados de teste..."; \
		bash scripts/seed-test-data.sh; \
		echo ""; \
		echo ">>> Servidor rodando em http://localhost:8080 (PID: $$SERVER_PID)"; \
		echo ">>> Para parar: kill $$SERVER_PID"; \
		wait $$SERVER_PID

# Testes
test:
	go test ./... -count=1

# Build da imagem de produção
docker:
	docker compose build

clean:
	rm -rf bin data
