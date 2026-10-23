# pocketbase-core — Plano de Implementação (Tasks)

Reescrita incremental da base técnica sobre PocketBase-como-framework. Cada task é demonstrável e testável, apoia-se nas anteriores e integra ao todo (sem código órfão). Refs apontam para [`requirements.md`](./requirements.md), [`design.md`](./design.md) e, ao fechar a onda, a fonte durável [`docs/requirements.md`](../../../docs/requirements.md) / [`docs/architecture.md`](../../../docs/architecture.md).

Convenções:
- `[ ]` pendente · `[~]` em progresso · `[x]` concluído.
- Cada task só é concluída com **build verde** (`make build`), **testes verdes** (`make check` quando aplicável) e **demo funcional** (smoke via HTTP e/ou tela).
- Toda mensagem ao usuário em PT-BR; valores em centavos; fuso `America/Sao_Paulo`.
- Toda regra sensível é imposta no servidor (defense-in-depth), mesmo com o `Pangolin` na frente; a UI apenas reflete.

**Definição de pronto (obrigatória em toda task — ver [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):**
- **Docs junto do código**: toda mudança de comportamento atualiza, no mesmo ciclo, `docs/requirements.md` (o quê), `docs/architecture.md` (como), `README.md` (se muda uso), Swagger (se muda contrato) e `docs/decisions.md` (se houve decisão relevante). Nunca "documentar depois".
- **Testes cobrem a mudança**: regra nova → unit no domínio/`internal/app`; fluxo/contrato novo → E2E sobre o binário único (Google/Gemini mockados); segurança/acesso (gate de aprovação) → E2E permitido **e** negado (2xx + 403/401); cripto → round-trip e não-vazamento do `SealedText`. Nenhum `t.Skip` sem justificativa no próprio teste.
- **Git por fase**: trabalhar na **branch da onda** (`pocketbase-core`). Ao concluir cada **Fase**, `make check` verde → **commit** em PT-BR imperativo referenciando a fase. **Push só no fim da onda** (após a Fase 10 verde); PR só então.

**Ordem das fases (dependências):** 0 (fundação) → 1 (crypto + `SealedText`) → 2 (domínio rico) → 3 (`platform/pb` + coleções) → 4 (API `/v1`) → 5 (auth + gate) → 6 (Calendar) → 7 (Drive fallback) → 8 (Gemini + tags) → 9 (portal do paciente) → 10 (fechamento: reescrever docs + release notes + push/PR).

---

## Fase 0 — Fundação pocketbase-core (R1)

- [ ] **0.1. Binário único com PocketBase embarcado** (dep: —)
  - `cmd/psicoman/main.go` com `app := pocketbase.New()` + registro de rotas no hook `app.OnServe()`.
  - `go.mod`: adicionar o PocketBase como dependência; remover os antigos `cmd/admin` e `cmd/portal`.
  - **Aceite:** `make build` produz **um** executável que sobe o PocketBase e serve a admin UI.
  - **Testes:** smoke de boot (sobe e responde health na porta configurada).
  - **Refs:** R1.1, R1.2 · D1.

- [ ] **0.2. Esqueleto de pacotes e Makefile** (dep: 0.1)
  - Criar `internal/platform/{crypto,pb}`, `internal/app`, `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}`, `internal/integration/{google,gemini}`, `internal/drivesync`, `internal/api` (pacotes vazios/esboço).
  - Makefile: `build` de **binário único** (saída = arquivo, não diretório), `check` (fmt + vet + build + testes), `test-unit`, `e2e`.
  - **Aceite:** `make check` passa no esqueleto; a regra de dependência (domínio não importa PB) está documentada no pacote.
  - **Testes:** `go vet`/build do esqueleto; teste trivial de que `internal/domain` não importa `platform/pb`.
  - **Refs:** R1.1, R5.1 · D0, D5.

- [ ] **Fechamento Fase 0:** `docs/decisions.md` já cobre a arquitetura (2026-10-21); conferir estrutura de pastas; `make check` verde → commit "pocketbase-core Fase 0: fundação do binário único com PocketBase embarcado".

---

## Fase 1 — `platform/crypto` + `SealedText` (R6)

- [ ] **1.1. Pacote `platform/crypto` (AES-256-GCM + `KeyProvider`)** (dep: 0.2)
  - `internal/platform/crypto`: `Seal`/`Open` com `crypto/aes`+`crypto/cipher`; `KeyProvider` (resolve chave por `id`, informa `CurrentID()`); montagem/parse do envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64).
  - **Aceite:** cifrar/decifrar round-trip; decifrar com `id_da_chave` antigo após trocar o corrente (rotação).
  - **Testes:** unit de round-trip, rotação por `id_da_chave`, nonce único por operação, falha ao decifrar com chave errada.
  - **Refs:** R6.2, R6.4 · D6, D11.

- [ ] **1.2. Tipo de domínio `SealedText`** (dep: 1.1)
  - `internal/domain` (tipo de valor compartilhado): `MarshalJSON` emite sempre o envelope cifrado; `UnmarshalJSON` faz parse e mantém cifrado; `Reveal(ctx)` decifra sob demanda; `Seal(ctx, plaintext)`.
  - **Aceite:** `MarshalJSON` nunca emite o claro; `Reveal` devolve o claro; `UnmarshalJSON(MarshalJSON(x))` preserva o envelope.
  - **Testes:** round-trip `Seal → MarshalJSON → UnmarshalJSON → Reveal`; **não-vazamento** (`MarshalJSON` só envelope); log do tipo não expõe o claro.
  - **Refs:** R6.1, R6.3, R6.5 · D6.

- [ ] **Fechamento Fase 1:** `docs/architecture.md` (cifra por campo), `docs/decisions.md` (se refinar envelope), Swagger não muda ainda; `make check` verde → commit "pocketbase-core Fase 1: SealedText e cripto por campo".

---

## Fase 2 — Domínio rico por bounded context (R4)

- [ ] **2.1. Agregados dos bounded contexts** (dep: 1.2)
  - `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}`: agregados com comportamento, validação, serialização e formatação de log (sem vazar clínico). Campos clínicos sensíveis em `clinical` usam `SealedText`.
  - Padrões: ULID em toda entidade; dinheiro `int64` centavos; instantes `America/Sao_Paulo` ISO-8601; idempotência de efeito financeiro.
  - **Aceite:** cada agregado valida-se e transita de estado por conta própria; serviços não concentram regra de domínio.
  - **Testes:** unit por context (validação, transições da sessão em `scheduling`, idempotência em `billing`, `SealedText` em `clinical`).
  - **Refs:** R4.1, R4.2, R4.3, R11.2 · D4.

- [ ] **2.2. Portas de repositório + serviços enxutos** (dep: 2.1)
  - Definir no domínio/app as **interfaces de porta** (repositórios) por context; `internal/app` com serviços enxutos que orquestram agregados + portas.
  - **Aceite:** o domínio não importa o PocketBase; os serviços dependem só das portas.
  - **Testes:** unit dos serviços com **fakes** das portas.
  - **Refs:** R4.1, R5.1 · D4, D5.

- [ ] **Fechamento Fase 2:** `docs/architecture.md` (bounded contexts + domínio rico); `make check` verde → commit "pocketbase-core Fase 2: domínio rico por bounded context".

---

## Fase 3 — `platform/pb` adapter + coleções/migrations (R5, R11)

- [ ] **3.1. Coleções do PocketBase com constraints/relations** (dep: 2.2)
  - Migrations do PocketBase (coleções por entidade) com UNIQUE (email/CPF), FK/relations entre entidades e chave única de idempotência financeira; campos clínicos como texto (recebem o envelope do `SealedText`).
  - **Aceite:** boot aplica as migrations em base nova; UNIQUE/FK/idempotência ativos.
  - **Testes:** migração no boot (base nova, idempotente); violar UNIQUE/chave de idempotência falha como esperado.
  - **Refs:** R11.1, R11.2 · D5, D13.

- [ ] **3.2. Adapter `platform/pb` (domínio↔Record)** (dep: 3.1)
  - `internal/platform/pb` implementa as portas de repositório, mapeando agregado↔`core.Record` (o envelope do `SealedText` vai como string na coluna). Único pacote que importa o PocketBase.
  - **Aceite:** CRUD por context round-trip via adapter; nada clínico em claro no banco (só envelope).
  - **Testes:** integração do adapter contra o PB embarcado (round-trip); teste de que o domínio não importa `platform/pb`.
  - **Refs:** R5.1, R5.2, R6.5 · D5, D6.

- [ ] **Fechamento Fase 3:** `docs/architecture.md` (coleções/migrations + adapter); `make check` verde → commit "pocketbase-core Fase 3: adapter platform/pb e coleções com constraints".

---

## Fase 4 — API `/v1` + envelope + Swagger + segregação lógica (R2, R7)

- [ ] **4.1. Router `/v1` + envelope + middleware chain** (dep: 3.2)
  - `internal/api`: grupos `/v1/admin/*` e `/v1/portal/*` sobre o router do PB (via `app.OnServe()`); envelope `{ message, elapsed_ms, data, error }` (PT-BR); middleware recover/request-id/timing/logging.
  - Restringir a REST automática do PocketBase para que **não** seja o contrato público.
  - **Aceite:** rotas `/v1` respondem no envelope padrão; `elapsed_ms` preenchido; REST automática não expõe o contrato.
  - **Testes:** E2E do envelope (sucesso e erro); teste de que a REST automática não serve dado clínico em claro.
  - **Refs:** R2.1, R7.1, R7.2 · D2, D7, D13.

- [ ] **4.2. Rotas de negócio + Swagger** (dep: 4.1)
  - Rotas por context (ex.: `POST /v1/admin/sessions`, `GET /v1/admin/patients/{id}/debts`), handlers finos que orquestram os serviços; Swagger/OpenAPI do `/v1`.
  - **Aceite:** rotas principais exercitadas por E2E; Swagger descreve o contrato.
  - **Testes:** E2E de contrato das rotas principais; validação do Swagger.
  - **Refs:** R7.1, R7.3, R11.1 · D7.

- [ ] **Fechamento Fase 4:** `docs/architecture.md` (contrato `/v1`), `docs/requirements.md`, Swagger; `make check` verde → commit "pocketbase-core Fase 4: contrato /v1 com envelope e segregação lógica".

---

## Fase 5 — Auth atrás do `Pangolin` + gate de aprovação (R3)

- [ ] **5.1. Auth do admin via header do `Pangolin`** (dep: 4.1)
  - Middleware no grupo `/v1/admin/*` que lê a identidade do header injetado pelo `Pangolin`; recusa (401) se ausente.
  - **Aceite:** requisição admin sem header é negada; com header é aceita.
  - **Testes:** E2E com e sem header (2xx/401).
  - **Refs:** R3.1 · D3.

- [ ] **5.2. Gate de aprovação do paciente (hook)** (dep: 5.1, 3.2)
  - Estado de aprovação (`pendente`/`aprovado`/`rejeitado`) no `patient`; hook do PocketBase impõe o gate no servidor; `/v1/portal/*` nega (403) paciente não-`aprovado` além do próprio cadastro.
  - **Aceite:** paciente `pendente`/`rejeitado` é barrado (403); `aprovado` acessa.
  - **Testes:** E2E permitido (2xx) e negado (403); unit do serviço de aprovação.
  - **Refs:** R3.2, R3.3 · D3, D13.

- [ ] **Fechamento Fase 5:** `docs/architecture.md`/`docs/requirements.md` (auth + gate), `docs/decisions.md` (gate via hook), Swagger; `make check` verde → commit "pocketbase-core Fase 5: auth atrás do Pangolin e gate de aprovação".

---

## Fase 6 — Integração Google Calendar síncrona (R8)

- [ ] **6.1. Calendar síncrono (interface + fake)** (dep: 4.2)
  - `internal/integration/google`: Calendar atrás de interface + fake; `freebusy` antes de confirmar; evento + Meet quando online; degrada sem panic sem credenciais.
  - **Aceite:** agendar/confirmar cria evento (mock); sem Google não dá erro fatal.
  - **Testes:** unit com fake; E2E agendar com Google mockado e sem Google.
  - **Refs:** R8.1 · D8.

- [ ] **Fechamento Fase 6:** `docs/architecture.md`/`docs/requirements.md` (Calendar síncrono), Swagger; `make check` verde → commit "pocketbase-core Fase 6: Google Calendar síncrono".

---

## Fase 7 — Drive fallback/DR via hook de backup (`drivesync`) (R9)

- [ ] **7.1. Sync de backup para o Drive** (dep: 3.1, 6.1)
  - `internal/drivesync`: hook no ciclo de **backup** do PocketBase dispara o upload do snapshot (cifrado) para pasta fria no Drive; escopo OAuth `drive.file`; fora do caminho de request; pode ser 2º processo.
  - **Aceite:** ao gerar backup, o snapshot é enviado ao Drive (mock); nenhuma rota de negócio toca o Drive.
  - **Testes:** unit com fake de Drive (upload chamado no hook de backup); E2E de backup sem Google não dá 500.
  - **Refs:** R9.1, R9.2 · D9.

- [ ] **Fechamento Fase 7:** `docs/architecture.md`/`docs/requirements.md` (Drive fallback/DR), `docs/decisions.md` (Drive fora do caminho de request); `make check` verde → commit "pocketbase-core Fase 7: Drive como fallback/DR via hook de backup".

---

## Fase 8 — IA Gemini + índice de tags (R10)

- [ ] **8.1. Gemini: resumo + tagueamento (opt-in, revisável)** (dep: 4.2, 2.1)
  - `internal/integration/gemini` (interface + fake): resumo clínico + tags; disparo explícito; resultado revisável antes de consolidar; opt-in por instância (default off); auditoria `ai.process` sem conteúdo clínico; minimização.
  - **Aceite:** com IA off, nada roda; com IA on, resumo/tags gerados e revisáveis; auditoria sem clínico.
  - **Testes:** unit com fake (minimização, auditoria sem clínico); E2E opt-in on/off.
  - **Refs:** R10.1, R10.3, R10.4 · D10.

- [ ] **8.2. Índice invertido de tags por paciente/ano** (dep: 8.1, 3.2)
  - Coleção/estrutura `tag -> lista de IDs de sessão`, por paciente e ano, isolada por paciente; tags em **claro** (indexáveis) enquanto o corpo é `SealedText`.
  - **Aceite:** consultar por tag devolve as sessões do paciente/ano; sem busca cross-paciente; corpo clínico permanece cifrado.
  - **Testes:** unit do índice (inserção/consulta por tag, isolamento por paciente); E2E de busca por tag.
  - **Refs:** R10.2, R6.5 · D10.

- [ ] **Fechamento Fase 8:** `docs/architecture.md`/`docs/requirements.md` (IA + índice de tags), `docs/decisions.md` (tags em claro, corpo cifrado), Swagger; `make check` verde → commit "pocketbase-core Fase 8: Gemini e índice invertido de tags".

---

## Fase 9 — Portal do paciente (R2, R3)

- [ ] **9.1. Portal mínimo (cadastro/login + recursos)** (dep: 5.2)
  - `/v1/portal/*`: cadastro/login (auth do PocketBase), recursos mínimos do paciente (próprios dados, agendamento quando previsto), respeitando o gate de aprovação.
  - **Aceite:** paciente cadastra, loga e acessa só o permitido; não-`aprovado` barrado.
  - **Testes:** E2E do fluxo de cadastro/login; permitido e negado pelo gate.
  - **Refs:** R2.2, R3.2, R3.3 · D2, D3.

- [ ] **Fechamento Fase 9:** `docs/requirements.md`/`README.md` (portal do paciente), Swagger; `make check` verde → commit "pocketbase-core Fase 9: portal do paciente".

---

## Fase 10 — Fechamento da onda

- [ ] **10.1. Reescrever docs canônicos para a nova arquitetura** (dep: 0–9)
  - Reescrever `docs/requirements.md` (o quê/por quê na arquitetura pocketbase-core) e `docs/architecture.md` (binário único, bounded contexts, `platform/pb`, `SealedText`, contrato `/v1`, Calendar síncrono, Drive fallback, Gemini + índice de tags). Atualizar `README.md` (uso: um binário atrás do `Pangolin`).
  - **Aceite:** docs canônicos descrevem a arquitetura da onda, sem resíduo das explorações descartadas.
  - **Testes:** revisão de coerência (links, nomenclatura canônica idêntica à ADR).
  - **Refs:** processo de entrega · todas as R#.

- [ ] **10.2. Release notes, `make check` e push/PR** (dep: 10.1)
  - `docs/release-notes.md` com a entrada `pocketbase-core` (Novidades: binário único com PocketBase embarcado, cifra por campo `SealedText`, contrato `/v1`, Drive como DR, Gemini opt-in com índice de tags; Melhorias: auth simplificada atrás do `Pangolin`; Notas de operação: um artefato, `data dir` do PB, chave de cifra via `KeyProvider`).
  - `make check` verde na branch inteira. **Push da branch `pocketbase-core`** (fim da onda) e abertura de PR para `main`.
  - **Aceite:** release notes publicadas; branch verde; PR aberto.
  - **Refs:** processo de entrega.

- [ ] **Fechamento Fase 10:** `docs/requirements.md`/`docs/architecture.md`/`README.md`/`docs/release-notes.md` atualizados; `make check` verde → commit "pocketbase-core Fase 10: reescrita das docs canônicas e fechamento da onda" → push + PR.
