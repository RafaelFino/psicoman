# pocketbase-finish — Plano de Implementação (Tasks)

Fecha as pendências e gaps da onda [`pocketbase-core`](../pocketbase-core/). Base técnica já estabelecida; aqui é **expor o que existe, completar o OAuth real, portar o perfil do terapeuta, observabilidade e docs operacionais**. Refs: [`requirements.md`](./requirements.md) (R1–R7), [`design.md`](./design.md) (D1–D7).

Convenções: `[ ]` pendente · `[~]` em progresso · `[x]` concluído. Cada task fecha com **build verde** (`make check`), **docs no mesmo ciclo** e **testes cobrindo** (unit/E2E; segurança → permitido+negado; cripto → round-trip+não-vazamento). Toda mensagem ao usuário em PT-BR; centavos; `America/Sao_Paulo`. Regra de dependência inalterada (`domain`/`app`/`config` não importam o PocketBase).

**Git:** branch da onda `pocketbase-finish`; commit por fase com `make check` verde; **push/PR só no fim e com OK do Rafael**.

---

## ⚠️ Antes de começar — decisões pendentes do Rafael

Esta spec **não deve ser implementada** até o Rafael resolver (ver `design.md`):
1. **⟨R1⟩** Recorte: todas as 4 sub-fases de R1 nesta onda, ou fatiar (sub-fases 3/4 → onda seguinte)?
2. **⟨R2⟩** O env `PSICOMAN_GOOGLE_REFRESH_TOKEN` continua como fallback de dev depois do fluxo OAuth real?
3. **⟨R4⟩** Verificação de email do portal: manter desligada (recomendado) ou ligar (exige transporte SMTP)?
4. **⟨R5⟩** Métricas: endpoint dedicado nesta onda ou só `/readyz`+`/livez` agora (métricas adiadas)?

A ordem e o escopo das fases abaixo assumem a recomendação do `design.md`; ajustar conforme as respostas.

---

## 📊 Status ao vivo da onda

> `✅ concluída` · `🔄 em progresso` · `⬜ pendente` · `⏸️ aguardando decisão`.

| Fase | Estado | Commit | Observação |
|------|--------|--------|------------|
| 0 — Spec (requirements/design/tasks) | ✅ concluída | _(pendente)_ | aguardando decisões do Rafael antes de implementar |
| 1 — Perfil do terapeuta (R3) | ⏸️ aguardando decisão | — | independente; destrava o cabeçalho da cobrança |
| 2 — OAuth 3-legged real + `google_token` (R2) | ⏸️ aguardando decisão | — | remove o seam de env; estado de reautorização |
| 3 — Observabilidade `/readyz` + `/livez` (R5) | ⏸️ aguardando decisão | — | `/readyz` reflete o OAuth (dep. Fase 2) |
| 4 — R1 sub-fase 1: núcleo utilizável (locais + planos) | ⏸️ aguardando decisão | — | destrava o débito por sessão via API |
| 5 — R1 sub-fase 2: telas iniciais (feed + história + métricas + calendário) | ⏸️ aguardando decisão | — | handlers finos sobre serviços existentes |
| 6 — R1 sub-fase 3: financeiro/custos (custos/ROI completo + relatório por local) | ⏸️ aguardando decisão | — | candidata a onda seguinte (⟨R1⟩) |
| 7 — R1 sub-fase 4: agendamento pelo portal (pedidos) | ⏸️ aguardando decisão | — | candidata a onda seguinte (⟨R1⟩) |
| 8 — Docs operacionais (R6) | ⏸️ aguardando decisão | — | dep. Fase 2 (descreve a autorização real) |
| 9 — Verificação de email do portal (R4) | ⏸️ aguardando decisão | — | só se o Rafael escolher a Opção B |
| 10 — Fechamento (docs canônicos + release notes + push/PR) | ⬜ pendente | — | push/PR só com OK do Rafael |

**Agora:** spec redigida (Fase 0). **Bloqueada aguardando as 4 decisões acima** antes de qualquer implementação.

---

## Fase 1 — Perfil do terapeuta (R3)

- [ ] **1.1 Porta + adapter + serviço `therapist`** (dep: —)
  - Porta `TherapistRepository` em `internal/app/ports.go`; adapter em `platform/pb` sobre `therapist_profile`/`therapist_platform_link`; `TherapistService` (Get/Upsert singleton 1:1, links, locais associados).
  - **Testes:** unit do serviço (upsert idempotente); adapter round-trip.
  - **Refs:** R3.1 · D3.
- [ ] **1.2 Rotas `/v1/app/therapist` + `/v1/portal/therapist` + cabeçalho na cobrança** (dep: 1.1)
  - `GET/PUT /v1/app/therapist`, `POST/DELETE /v1/app/therapist/platform-links`; `GET /v1/portal/therapist` (só `PublicView()`). `InvoiceService.compose` passa a imprimir nome/CRP.
  - **Testes:** E2E do contrato; E2E da cobrança com cabeçalho; portal vê só público.
  - **Refs:** R3.2–R3.4 · D3.
- [ ] **Fechamento Fase 1:** docs (architecture + Swagger), `make check` verde → commit.

---

## Fase 2 — OAuth 3-legged real + `google_token` cifrado (R2)

- [ ] **2.1 Adapter `google_token` (token cifrado na borda)** (dep: —)
  - Porta `GoogleTokenRepository` (`Save`/`Load`); adapter em `platform/pb` cifrando `token` via `Cipher` (envelope); `scopes`/`expiry` em claro.
  - **Testes:** round-trip + não-vazamento do token.
  - **Refs:** R2.1–R2.2 · D2.
- [ ] **2.2 Fluxo de autorização (admin)** (dep: 2.1)
  - `GET /v1/admin/google/authorize` (URL de consentimento, `access_type=offline`, `prompt=consent`, escopos `calendar`+`drive.file`) e `GET /v1/admin/google/callback` (troca code→refresh token, persiste cifrado).
  - **Testes:** E2E do callback com servidor OAuth fake (sem Google real).
  - **Refs:** R2.1 · D2.
- [ ] **2.3 Boot lê token do repo + estado de reautorização** (dep: 2.1, 2.2)
  - `wireCalendar`/`wireDriveSync` carregam o refresh token do `google_token` (env como fallback de dev conforme ⟨R2⟩); renovação automática; falha → estado "reautorização necessária" sem panic.
  - **Testes:** boot lê do repo; degradação observável quando ausente.
  - **Refs:** R2.3–R2.4 · D2.
- [ ] **Fechamento Fase 2:** docs + decisions (fluxo OAuth), `make check` verde → commit.

---

## Fase 3 — Observabilidade `/readyz` + `/livez` (R5)

- [ ] **3.1 `/readyz` e `/livez`** (dep: 2.3)
  - `/readyz`: cifra montada + DB acessível + estado do OAuth Google (ok|degraded). `/livez`: 200 fixo.
  - **Testes:** E2E (readyz ok; readyz degraded sem token Google; livez 200).
  - **Refs:** R5.1 · D5.
- [ ] **3.2 Métricas** (dep: —) — **⟨R5⟩** só se o Rafael optar por endpoint nesta onda.
  - **Refs:** R5.2 · D5.
- [ ] **Fechamento Fase 3:** docs, `make check` verde → commit.

---

## Fase 4 — R1 sub-fase 1: núcleo utilizável (locais + planos)

- [ ] **4.1 Locais, disponibilidades e origens** (dep: —) — CRUD `/v1/app/locations`, `/availability`, `/v1/app/origins`. **Refs:** R1.7 · D1.
- [ ] **4.2 Planos e pagamentos** (dep: 4.1) — CRUD `/v1/app/patients/{id}/plans`; comprovante de pagamento no GED. Com plano vigente, o débito por sessão passa a funcionar ponta a ponta via API. **Refs:** R1.9 · D1.
- [ ] **Fechamento Fase 4:** E2E ponta a ponta (cadastrar plano → finalizar sessão → débito gerado → cobrança), docs + Swagger, `make check` verde → commit.

---

## Fase 5 — R1 sub-fase 2: telas iniciais

- [ ] **5.1 Feed do dia** `GET /v1/app/day-feed` (dep: —). **Refs:** R1.1.
- [ ] **5.2 História do paciente** `GET /v1/app/patients/{id}/history` (auditada, sem clínico em metadados) (dep: —). **Refs:** R1.2.
- [ ] **5.3 Métricas de sessão** `GET /v1/app/patients/{id}/session-metrics` (dep: —). **Refs:** R1.3.
- [ ] **5.4 Calendário mensal** `GET /v1/app/sessions?patient_id=&from=&to=` (dep: —). **Refs:** R1.4.
- [ ] **Fechamento Fase 5:** E2E de contrato de cada rota + não-vazamento clínico, docs + Swagger, `make check` verde → commit.

---

## Fase 6 — R1 sub-fase 3: financeiro/custos — **⟨R1⟩ candidata a onda seguinte**

- [ ] **6.1 Custos/ROI completo** (CRUD `cost_*` + relatório de custo) (dep: 4.1). **Refs:** R1.6.
- [ ] **6.2 Relatório de sessões por local** `GET /v1/app/locations/{id}/session-report` (dep: 4.1). **Refs:** R1.5.
- [ ] **Fechamento Fase 6:** E2E + docs, `make check` verde → commit.

---

## Fase 7 — R1 sub-fase 4: agendamento pelo portal — **⟨R1⟩ candidata a onda seguinte**

- [ ] **7.1 Pedidos de agendamento** `POST /v1/portal/appointment-requests`, `GET /v1/app/appointment-requests`, `POST /v1/app/appointment-requests/{id}/confirm` (cria a sessão) (dep: 4.1). O hook de aprovação já reforça. **Refs:** R1.8 · D1.
- [ ] **Fechamento Fase 7:** E2E permitido (aprovado) e negado (pendente 403), docs + Swagger, `make check` verde → commit.

---

## Fase 8 — Docs operacionais (R6)

- [ ] **8.1 Reescrever `docs/deploy.md`** para o binário único (serve/dir, Pangolin, `config.yaml`, autorização OAuth da Fase 2, systemd, atualização segura). Remover a nota de "desatualizado". (dep: Fase 2) **Refs:** R6.1, R6.3.
- [ ] **8.2 Reescrever `docs/stack-local.md`** (stack Docker do binário único). Remover a nota. **Refs:** R6.2, R6.3.
- [ ] **Fechamento Fase 8:** revisão de coerência (links, comandos reais), commit.

---

## Fase 9 — Verificação de email do portal (R4) — **⟨R4⟩ só se Opção B**

- [ ] **9.1** Ligar a verificação de email nativa do PocketBase + transporte SMTP; fluxo de confirmação antes da aprovação. (dep: —) **Refs:** R4.1 · D4.
- [ ] **Fechamento Fase 9:** E2E do fluxo, docs, `make check` verde → commit.

---

## Fase 10 — Fechamento da onda

- [ ] **10.1** Atualizar `docs/requirements.md`/`docs/architecture.md`/`README.md` com as novas rotas/capacidades; `docs/release-notes.md` com a entrada `pocketbase-finish` (orientada ao usuário).
- [ ] **10.2** `docs/decisions.md` com as decisões tomadas; Swagger completo.
- [ ] **10.3** `make check` verde na branch inteira → **PARAR e confirmar com o Rafael antes de push/PR**.
