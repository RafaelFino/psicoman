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

- Cada teste E2E sobe o binário relevante (`admin` ou `portal`) com um SQLite temporário (arquivo novo por teste, nunca compartilhado entre testes) e mocks das integrações Google (`CalendarClient`, `GmailClient`, `DriveClient` fakes).
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
