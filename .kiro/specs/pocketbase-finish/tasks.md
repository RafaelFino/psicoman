# pocketbase-finish — Plano de Implementação (Tasks)

Fecha as pendências e gaps da onda [`pocketbase-core`](../pocketbase-core/). Base técnica já estabelecida; aqui é **expor o que existe, completar o OAuth real, portar o perfil do terapeuta, observabilidade e docs operacionais**. Refs: [`requirements.md`](./requirements.md) (R1–R7), [`design.md`](./design.md) (D1–D7).

Convenções: `[ ]` pendente · `[~]` em progresso · `[x]` concluído. Cada task fecha com **build verde** (`make check`), **docs no mesmo ciclo** e **testes cobrindo** (unit/E2E; segurança → permitido+negado; cripto → round-trip+não-vazamento). Toda mensagem ao usuário em PT-BR; centavos; `America/Sao_Paulo`. Regra de dependência inalterada (`domain`/`app`/`config` não importam o PocketBase).

**Git:** branch da onda `pocketbase-finish`; commit por fase com `make check` verde; **push/PR só no fim e com OK do Rafael**.

---

## ✅ Decisões do Rafael (2026-10-24) — resolvidas

1. **⟨R1⟩ Recorte:** **fatiar.** Sub-fases 1 e 2 (Fases 4 e 5) entram nesta onda; sub-fases 3 e 4 (Fases 6 e 7) vão para onda seguinte. Entrega o "produto utilizável ponta a ponta" (locais+planos+feed+história) num escopo coeso.
2. **⟨R2⟩ Env fallback:** **manter como fallback só em dev, com precedência do repo.** O `google_token` cifrado é o caminho primário; o env `PSICOMAN_GOOGLE_REFRESH_TOKEN` só entra quando o repo está vazio, e o boot loga a origem.
3. **⟨R4⟩ Verificação de email:** **manter desligada (Opção A).** O gate de aprovação humana já é o controle de acesso efetivo; ligar SMTP não fecha buraco e adiciona infra. Fase 9 fora de escopo.
4. **⟨R5⟩ Métricas:** **só `/readyz`+`/livez` nesta onda; endpoint de métricas adiado** (timing por rota já é logado). Registrado como adiamento consciente em `docs/decisions.md`.

**Diretriz transversal (Rafael):** testes em TODA a plataforma — unit, E2E de jornada, mocks de tudo que é externo (Google, OAuth). Muitas mudanças pesadas de arquitetura foram feitas; validar que tudo funciona é prioridade. Gate obrigatório por fase.

**Escopo da onda:** Fases 1, 2, 3, 4, 5, 8, 10. **Fora de escopo** (onda seguinte): Fases 6, 7, 9.

---

## 📊 Status ao vivo da onda

> `✅ concluída` · `🔄 em progresso` · `⬜ pendente` · `🚫 fora de escopo`.

| Fase | Estado | Commit | Observação |
|------|--------|--------|------------|
| 0 — Spec (requirements/design/tasks) | ✅ concluída | `ac3519d` | decisões resolvidas em 2026-10-24 |
| 1 — Perfil do terapeuta (R3) | ✅ concluída | `2e84663` | porta+adapter+serviço+rotas+cabeçalho na cobrança; make check verde |
| 2 — OAuth 3-legged real + `google_token` (R2) | ✅ concluída | `0b5ca33` | fluxo admin authorize/callback + token cifrado + precedência do repo; make check verde |
| 3 — Observabilidade `/readyz` + `/livez` (R5) | ✅ concluída | _(a seguir)_ | cifra+DB+OAuth; métricas adiadas; make check verde |
| 4 — R1 sub-fase 1: núcleo utilizável (locais + planos) | ⬜ pendente | — | destrava o débito por sessão via API |
| 5 — R1 sub-fase 2: telas iniciais (feed + história + métricas + calendário) | ⬜ pendente | — | handlers finos sobre serviços existentes |
| 6 — R1 sub-fase 3: financeiro/custos | 🚫 fora de escopo | — | adiada p/ onda seguinte (⟨R1⟩) |
| 7 — R1 sub-fase 4: agendamento pelo portal | 🚫 fora de escopo | — | adiada p/ onda seguinte (⟨R1⟩) |
| 8 — Docs operacionais (R6) | ⬜ pendente | — | dep. Fase 2 (descreve a autorização real) |
| 9 — Verificação de email do portal (R4) | 🚫 fora de escopo | — | ⟨R4⟩ Opção A: gate de aprovação basta |
| 10 — Fechamento (docs canônicos + release notes + push/PR) | ⬜ pendente | — | push/PR só com OK do Rafael |

**Agora:** Fase 3 (observabilidade) concluída e verde — commit a seguir. **Próximo:** Fase 4 (R1 sub-fase 1: locais + disponibilidades + origens + planos + pagamentos → débito por sessão ponta a ponta via API).

---

## Fase 1 — Perfil do terapeuta (R3)

- [x] **1.1 Porta + adapter + serviço `therapist`** (dep: —)
  - Porta `TherapistRepository` em `internal/app/ports.go`; adapter em `platform/pb` sobre `therapist_profile`/`therapist_platform_link`; `TherapistService` (Get/Upsert singleton 1:1, links, locais associados).
  - **Testes:** unit do serviço (upsert idempotente); adapter round-trip. ✅
  - **Refs:** R3.1 · D3.
- [x] **1.2 Rotas `/v1/app/therapist` + `/v1/portal/therapist` + cabeçalho na cobrança** (dep: 1.1)
  - `GET/PUT /v1/app/therapist`, `POST/DELETE /v1/app/therapist/platform-links`; `GET /v1/portal/therapist` (só `PublicView()`). `InvoiceService.compose` passa a imprimir nome/CRP.
  - **Testes:** E2E do contrato; E2E da cobrança com cabeçalho; portal vê só público. ✅
  - **Refs:** R3.2–R3.4 · D3.
- [x] **Fechamento Fase 1:** docs (architecture §0.13 + decisions + Swagger), `make check` verde → commit.

---

## Fase 2 — OAuth 3-legged real + `google_token` cifrado (R2)

- [x] **2.1 Adapter `google_token` (token cifrado na borda)** (dep: —)
  - Porta `GoogleTokenRepository` (`Save`/`Load`); adapter em `platform/pb` cifrando `token` via `Cipher` (envelope); `scopes`/`expiry` em claro. Domínio `GoogleToken` com refresh token `SealedText`.
  - **Testes:** round-trip + não-vazamento do token. ✅
  - **Refs:** R2.1–R2.2 · D2.
- [x] **2.2 Fluxo de autorização (admin)** (dep: 2.1)
  - `GET /v1/admin/google/authorize` (URL de consentimento, `access_type=offline`, `prompt=consent`, escopos `calendar`+`drive.file`), `GET /v1/admin/google/callback` (troca code→refresh token, persiste cifrado) e `GET /v1/admin/google/status`.
  - **Testes:** E2E in-process do callback com servidor OAuth fake (sem Google real). ✅
  - **Refs:** R2.1 · D2.
- [x] **2.3 Boot lê token do repo + estado de reautorização** (dep: 2.1, 2.2)
  - `wireCalendar`/`wireDriveSync` carregam o refresh token do `google_token` (env fallback de dev, precedência do repo ⟨R2⟩); falha → `GoogleAuthService` degraded ("reautorização necessária") sem panic.
  - **Testes:** `GoogleAuthService` status/precedência/reauth (unit); degradação observável. ✅
  - **Refs:** R2.3–R2.4 · D2.
- [x] **Fechamento Fase 2:** docs (architecture §0.14 + decisions + Swagger), `make check` verde → commit.

---

## Fase 3 — Observabilidade `/readyz` + `/livez` (R5)

- [x] **3.1 `/readyz` e `/livez`** (dep: 2.3)
  - `/readyz`: cifra montada (round-trip) + DB acessível (lookup) + estado do OAuth Google (ok|degraded; não bloqueia prontidão). `/livez`: 200 fixo. 503 se cifra/DB falham.
  - **Testes:** E2E (livez 200; readyz pronto com google=degraded sem token) + in-process (readyz google=ok após callback OAuth fake; não-vazamento). ✅
  - **Refs:** R5.1 · D5.
- [x] **3.2 Métricas** — **⟨R5⟩ ADIADO** (decisão do Rafael): endpoint de métricas fica para onda futura; timing por rota já logado cobre o mínimo. Registrado em `docs/decisions.md`.
  - **Refs:** R5.2 · D5.
- [x] **Fechamento Fase 3:** docs (architecture §0.15 + decisions + Swagger), `make check` verde → commit.

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
