# Psicoman — Convenções do Projeto

## Arquitetura

- Backend: Go 1.23 (Gin framework), SQLite (modernc.org/sqlite, pure-Go), JWT para pacientes, Pangolin headers para psicólogo
- Frontend: Go html/template + htmx 2.x + Alpine.js 3.x (embutido no binário via go:embed)
- Monolito: um único serviço Go serve a API REST (JSON) e as páginas HTML (templates)
- Camadas Go: `cmd/server` → `internal/web` → `internal/service` → `internal/storage` → `internal/domain`
- Sem Node.js, sem npm, sem React — decisão definitiva, não revisitar

## Modo de desenvolvimento

- `DEV_MODE=true` habilita rotas em `/api/dev/*` e autenticação via header `X-Dev-Auth`
- O secret padrão em dev é `dev-local` (consistente entre backend e docker-compose.dev.yml)
- Backend dev: `make run-api` (lê `.env.dev` para variáveis de ambiente)
- Alternativamente: `make run` (Docker Compose dev)
- Seed de dados de teste: `bash scripts/seed-test-data.sh`
- Cleanup de dados de teste: `curl -X DELETE localhost:8080/api/dev/test-data -H "X-Dev-Auth: dev-local"`

## Contrato API (JSON)

- Todos os campos JSON usam **snake_case** (ver steering `go-json-serialization.md`)
- Listas vazias retornam `null` em Go (não `[]`) — templates devem tratar com `{{if .List}}`
- Datas são sempre em RFC3339 UTC: `"2026-07-22T14:00:00Z"`
- IDs são UUIDs string
- Erros retornam `{"error": "mensagem"}`
- Valores monetários em centavos (int64): `amount_cents`, `cost_cents`

## Interface (UI)

- Design system: CSS custom com variáveis, sem bibliotecas externas
- Tema: claro e escuro (toggle manual + auto via prefers-color-scheme)
- Layout: sidebar + header + main (psicólogo), header + main (paciente)
- Responsividade: mobile-first, breakpoint principal 768px
- Touch targets: mínimo 44px em telas touch
- Interatividade: htmx para AJAX parcial, Alpine.js para estado local (modais, tabs)
- Sem SPA — cada página é server-rendered, htmx faz atualizações parciais

## Docker e permissões

- Docker Compose mapeia `./data/db`, `./data/ged`, `./data/logs` como volumes do host
- Dockerfile: single-stage Go build (Alpine), ~20MB
- Ao recriar dados de teste: `rm data/db/*.sqlite*` e reiniciar

## Testes

- `go test ./...` deve sempre passar antes de considerar uma mudança pronta
- Testes de integração HTTP: `internal/web/handlers_test.go`
- Testes de domínio/regras: `internal/domain/scheduling_test.go`
- Testes de storage: `internal/storage/storage_test.go`
- Dados de teste usam prefixo "TEST " no nome e @test.com no email

## Documentação

- Diagramas sempre em **Mermaid** (nunca ASCII art)
- Docs em português brasileiro
- Estrutura: README.md (overview), docs/release-notes.md, docs/next-steps.md, docs/testing.md
