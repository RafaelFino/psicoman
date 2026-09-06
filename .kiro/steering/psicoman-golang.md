---
inclusion: fileMatch
fileMatchPattern: "**/*.go,**/go.mod,**/Makefile"
---

# Psicoman — Convenções Go do projeto

Complementa o steering global de Go (idiomas, estilo, concorrência). Aqui ficam as decisões **específicas do Psicoman**, definidas em `docs/architecture.md`. Sempre consultar esse documento antes de desviar de um padrão aqui descrito.

## Estrutura e camadas

- Layout: `cmd/admin`, `cmd/portal`, `internal/{domain,repository,service,integration,api,web,config,platform,migration}` (ver `docs/architecture.md §2`).
- Regra de dependência **unidirecional**: `domain` não importa nada do projeto; `repository`/`service`/`integration` importam `domain`; `api` importa `service`; `web` é servido pela `api`. Nunca importar de uma camada "de fora" para "dentro" (ex: `domain` importando `service`).
- **Domínio magro**: entidades em `internal/domain` carregam dados e invariantes básicas (validação de shape), não orquestração de negócio. Orquestração vive em `internal/service`.
- Dois binários (`cmd/admin`, `cmd/portal`) compartilham `internal/*`. Código que é exclusivo de um binário (ex: middleware de auth do Pangolin) fica em `internal/api/admin` ou `internal/api/portal`, nunca em pacotes compartilhados.

## Padrões de domínio (não negociáveis)

- **IDs**: ULID em toda entidade nova (`internal/platform/ulid`). Nunca `int` autoincrement nem `uuid.New()` puro.
- **Dinheiro**: sempre `int64` em centavos, moeda BRL implícita. Nunca `float64` para valores monetários — em cálculo, divisão ou arredondamento envolvendo dinheiro, usar inteiro e documentar a regra de arredondamento (ex: rateio de custo, `docs/architecture.md §4.1`).
- **Tempo**: `time.Time` sempre em `America/Sao_Paulo` (não usar `time.Now()` puro sem carregar a location; usar um helper central, ex: `platform/clock`). Persistir como texto ISO-8601 no SQLite.
- **Idempotência**: qualquer operação que gera efeito financeiro (débito) precisa de `idempotency_key` e usar `INSERT ... ON CONFLICT DO NOTHING` (ou equivalente) — nunca "checar antes de inserir" com race condition entre check e insert.
- **Erros para o usuário**: toda mensagem de erro retornada pela API é em **PT-BR**, mesmo que o erro interno/log seja em inglês. Não deixar `err.Error()` (que costuma vir da lib em inglês) vazar direto pro body da resposta.

## Contexto e cancelamento

- Toda função de camada `service`/`repository`/`integration` que faz I/O recebe `context.Context` como primeiro parâmetro.
- Chamadas à API do Google (Calendar/Gmail/Drive) sempre com timeout via `context.WithTimeout` — nunca sem timeout, para não travar o handler HTTP esperando uma API externa.

## Testes

- Regras de negócio sensíveis (idempotência de débito, rateio de custo, gatilho de plano fechado, checagem de conflito de agenda) sempre com teste unitário cobrindo o caso feliz **e** o caso de borda (dupla chamada, período vazio, conflito exato no limite do horário).
- Ver `psicoman-testes-e2e.md` para a convenção de E2E.

## Middleware / contrato de API

- Toda rota HTTP passa pela chain `recover → request-id → timing → logging → auth → handler` (`docs/architecture.md §5`). Novas rotas não pulam etapas dessa chain.
- Toda resposta usa o envelope padrão (`message`, `elapsed_ms`, `data`, `error`). Não introduzir um formato de resposta alternativo sem atualizar `docs/architecture.md §5` primeiro.
