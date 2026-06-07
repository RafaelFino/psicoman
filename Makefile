.PHONY: frontend build run run-api run-dev test docker clean

# Build do frontend (React → dist → internal/web/static)
frontend:
	cd frontend && npm install && npm run build
	rm -rf internal/web/static && cp -r frontend/dist internal/web/static

# Build completo: frontend embutido no binário Go
build: frontend
	go build -tags embedfrontend -o bin/psicoman ./cmd/server

# Build só da API (sem frontend embutido — para dev com vite separado)
build-api:
	go build -o bin/psicoman ./cmd/server

# ─────────────────────────────────────────────────────────────────
# make run → sobe o ambiente dev completo via Docker Compose.
# Inclui DEV_MODE=true, frontend embutido, volumes locais mapeados.
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
run-api: build-api
	@echo ">>> DEV MODE — rotas /api/dev/* ativas"
	env $$(grep -v '^#' .env.dev | grep -v '^$$' | xargs) ./bin/psicoman

# Testes
test:
	go test ./... -count=1

# Build da imagem de produção
docker:
	docker compose build

clean:
	rm -rf bin frontend/dist frontend/node_modules data
