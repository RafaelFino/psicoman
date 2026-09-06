---
inclusion: fileMatch
fileMatchPattern: "**/*.sql,**/repository/sqlite/**,**/migrations/**"
---

# Psicoman — SQLite e persistência

Convenções específicas do projeto para o schema e o acesso a dados. Modelo completo em `docs/architecture.md §3`.

## Modo de operação

- **WAL mode** obrigatório (`PRAGMA journal_mode=WAL`) — os dois binários (`admin` e `portal`) abrem o mesmo arquivo `.db` concorrentemente.
- `PRAGMA busy_timeout` configurado (ex: 5000ms) para absorver contenção pontual de escrita entre os dois processos.
- `PRAGMA foreign_keys = ON` sempre — o schema depende de integridade referencial (paciente → sessão → débito, etc.).
- Escrita do `psicoman-portal` é mínima por design (cadastro básico + pedido de agendamento). Se uma nova feature do portal exigir escrita pesada, revisitar a decisão de storage compartilhado em `docs/architecture.md §9` antes de implementar.

## Convenções de schema

- PK sempre `id TEXT` com valor ULID gerado em Go (não `AUTOINCREMENT`, não `INTEGER PRIMARY KEY` puro).
- Timestamps (`created_at`, `updated_at`) em `TEXT` ISO-8601, fuso `America/Sao_Paulo`, gerados pela aplicação — não usar `DEFAULT CURRENT_TIMESTAMP` do SQLite (que é UTC).
- Valores monetários em `INTEGER` (centavos). Nunca `REAL`/`FLOAT` para dinheiro.
- Soft-delete via `deleted_at TEXT NULL` nas entidades que o exigem (ver `docs/architecture.md §3.2` — hoje: paciente, sessão, débito). Índices/consultas de listagem sempre filtram `deleted_at IS NULL`.
- Chaves de idempotência (`idempotency_key`) sempre `UNIQUE`, nunca validadas apenas em nível de aplicação.
- Toda constraint de unicidade de negócio (email do paciente, cpf do paciente, `idempotency_key`) vira `UNIQUE` no schema — não confiar só em validação da camada de serviço.

## Migrations

- Uma migration por mudança de schema, numerada e versionada em `migrations/`, aplicada automaticamente no boot (`internal/migration`).
- Migrations são **append-only**: nunca editar uma migration já aplicada em qualquer ambiente; criar uma nova para corrigir.
- Toda migration que adiciona uma coluna `NOT NULL` a uma tabela existente precisa de um `DEFAULT` ou de um passo de backfill explícito.

## Repositórios

- Uma implementação de repositório por agregado (ex: `PatientRepository`, `SessionRepository`), sempre atrás de uma interface definida em `internal/service` ou `internal/domain` (para permitir fake nos testes).
- Nenhum SQL solto em `service`; toda query vive em `internal/repository/sqlite`.
- Queries de relatório (financeiro, custos, ROI) podem ser mais elaboradas (joins, agregações), mas ainda seguem através de um método de repositório nomeado pelo caso de uso (ex: `ReportRepository.FinancialSummary(...)`), não SQL ad-hoc espalhado pela API.
