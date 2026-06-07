.PHONY: frontend build run run-dev test docker clean

frontend:
	cd frontend && npm install && npm run build
	rm -rf internal/web/static && cp -r frontend/dist internal/web/static

build: frontend
	go build -tags embedfrontend -o bin/psicoman ./cmd/server

build-api:
	go build -o bin/psicoman ./cmd/server

run: build-api
	DATA_DIR=./data ./bin/psicoman

run-dev: build-api
	@echo ">>> DEV MODE — rotas /api/dev/* ativas, X-Dev-Auth requerido"
	@echo ">>> DEV_SECRET=$(DEV_SECRET)"
	env $$(cat .env.dev | grep -v '^#' | xargs) ./bin/psicoman

test:
	go test ./... -count=1

docker:
	docker compose build

clean:
	rm -rf bin frontend/dist frontend/node_modules data
