---
inclusion: fileMatch
fileMatchPattern: "**/test/e2e/**,**/*_test.go"
---

# Psicoman — Estratégia de testes

Convenção de testes do projeto. Ver `docs/architecture.md §7`.

## Unitários

- Regras de negócio isoladas (idempotência de débito, rateio de custo, gatilho por tipo de plano, ROI por canal, validação de CPF/email únicos) sempre com teste unitário no pacote `service`, usando fakes de repositório/integração (interfaces definidas em `domain`/`service`).
- Table-driven tests para regras com múltiplas variações (ex: os 5 tipos de plano, os 4 tipos de periodicidade de custo de local).

## E2E (`test/e2e/`)

- Cada teste E2E sobe o **binário único** `cmd/psicoman` (PocketBase embarcado; os atores `app`, `portal` e `admin` são prefixos lógicos do mesmo processo) com um SQLite temporário (arquivo novo por teste, nunca compartilhado entre testes). O binário real não aceita injetar fakes: o Google fica sem credenciais (degrada) e os fluxos com Google/Gemini mockados são cobertos **em processo** em `internal/api` (seams `calendarWirer`, `aiSummarizerBuilder`, `driveSyncWirer`, com `FakeDriveClient`/fakes das portas).
- Cobrem fluxo de negócio completo via API (request HTTP real), não chamada direta de função — o objetivo é validar a chain de middleware + service + repository junto.
- Fluxos mínimos que **sempre** precisam de cobertura E2E (um por task da `.kiro/specs/mvp/tasks.md`, no mínimo):
  - Cadastro → sessão → confirmação (com e sem conflito de agenda) → finalização → débito → PDF → quitação.
  - Pedido de agendamento (portal) → pendência (admin) → confirmação → evento no Calendar.
  - Job de fechamento de ciclo para plano fechado gerando débito sem sessão associada.
  - Backup → restore round-trip.
  - Acesso negado ao admin sem header/secret válidos; acesso do paciente restrito aos próprios dados.
- Testes E2E não dependem de rede externa nem de credenciais reais do Google — 100% mockado.

## Antes de marcar uma task como concluída

- `make test` (unit) e `make e2e` verdes.
- Nenhum teste "pulado" (`t.Skip`) sem justificativa registrada no próprio teste.

## Métricas (Prometheus) e rate limit

- **E2E de métricas** (`test/e2e/metrics_test.go`): sobe o binário com o listener dedicado (`PSICOMAN_METRICS_ADDR` em `freePort` extra, helper `startServerWithMetrics`) e cobre, no mínimo: (1) exposição no formato Prometheus; (2) tráfego real gerando as séries com `rota` = **padrão registrado** (`{id}`, nunca o id real) e `motivo` de 401/403 — **permitido e negado juntos** (inclui o 403 real do portal com sessão de paciente não aprovado); (3) **vazamento**: sentinelas de nome/e-mail/CPF/telefone/nota clínica, o ULID real, o valor de `X-Pangolin-User` e fragmento da chave de cifra **não** podem aparecer em `/metrics`, e os valores de `ator`/`metodo`/`status` ficam nos enums; (4) **isolamento**: a API responde 404 a `/metrics` e `/v1/metrics`, e o listener responde 404 a tudo que não é `GET /metrics`; (5) **fail-closed**: sem `PSICOMAN_METRICS_ADDR` nenhum listener extra abre.
- Qualquer família nova de métrica exige teste de **não-vazamento** (rótulos só de enums fechados) e, se for contador de negócio, teste de que **só o caminho de sucesso** conta.
- **E2E de rate limit** (`test/e2e/portal_test.go`): N tentativas dentro do limite não podem ser `429`; a seguinte é `429` com envelope PT-BR (`limite_excedido`) e `Retry-After`; outra identidade segue normal; o corpo do `429` **não ecoa** o e-mail. O limite por paciente (agendamento) é coberto **em processo** (`internal/api/portal_ratelimit_test.go`) porque exige sessão de paciente aprovado.
- Testes de OAuth do Google passam pelo fluxo real **authorize → cookie `psicoman_oauth_state` → callback**; nunca chamam o callback sem passar pelo authorize.
