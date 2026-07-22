# Psicoman — Convenções do Projeto

## Arquitetura

- Backend: Go (Gin framework), SQLite (modernc.org/sqlite), JWT para pacientes, Pangolin headers para psicólogo
- Frontend: React 18, React Router, Vite
- Monolito: um único serviço Go serve a API e o frontend (embed em produção, proxy em dev)
- Camadas Go: `cmd/server` → `internal/web` → `internal/service` → `internal/storage` → `internal/domain`

## Modo de desenvolvimento

- `DEV_MODE=true` habilita rotas em `/api/dev/*` e autenticação via header `X-Dev-Auth`
- O secret padrão em dev é `dev-local` (consistente entre backend, docker-compose.dev.yml e frontend)
- Frontend dev roda via `cd frontend && npm run dev` (Vite em :5173, proxy para Go em :8080)
- Backend dev: `make run-api` (lê `.env.dev` para variáveis de ambiente)

## Contrato Frontend ↔ Backend

- Todos os campos JSON usam **snake_case** (ver steering `go-json-serialization.md`)
- Listas vazias retornam `null` em Go (não `[]`) — frontend deve tratar com `?.` ou `|| []`
- Datas são sempre em RFC3339 UTC: `"2026-07-22T14:00:00Z"`
- IDs são UUIDs string
- Erros retornam `{"error": "mensagem"}`

## Docker e permissões

- Docker Compose mapeia `./data/db`, `./data/ged`, `./data/logs` como volumes do host
- Arquivos criados pelo container Docker rodam como root — ao rodar fora do Docker, garantir que o usuário local tem permissão de escrita nesses diretórios
- Ao recriar dados de teste, deletar `data/db/*.sqlite*` e deixar o servidor recriar na próxima execução

## Testes

- `go test ./...` deve sempre passar antes de considerar uma mudança pronta
- Testes de integração HTTP ficam em `internal/web/handlers_test.go`
- Testes de domínio/regras em `internal/domain/scheduling_test.go`
- Testes de storage em `internal/storage/storage_test.go`

## Seed de dados

- Script de seed: `bash scripts/seed.sh` (requer servidor rodando em :8080 com DEV_MODE)
- Para reset completo: parar servidor → `rm data/db/dev.sqlite*` → reiniciar → rodar seed
