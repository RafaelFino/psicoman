# prontuario-ux — Plano de Implementação (Tasks)

Execução incremental. Cada task é demonstrável e testável, apoia-se nas anteriores e integra ao todo (sem código órfão). Refs apontam para [`requirements.md`](./requirements.md), [`design.md`](./design.md) e as fontes duráveis [`docs/requirements.md`](../../../docs/requirements.md) e [`docs/architecture.md`](../../../docs/architecture.md).

Convenções:
- `[ ]` pendente · `[~]` em progresso · `[x]` concluído.
- Cada task só é concluída com **build verde**, **testes verdes** e **demo funcional** (smoke via HTTP e/ou tela).
- Toda mensagem ao usuário em PT-BR; sem `window.prompt/confirm`; WCAG AA (foco visível, alvo ≥ 44px) nos controles novos.
- Regra de negócio imposta no servidor; a UI apenas reflete.

**Definição de pronto (obrigatória em toda task — ver [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):**
- **Docs junto do código**: toda mudança de comportamento atualiza, no mesmo ciclo, `docs/requirements.md` (o quê), `docs/architecture.md` (como), `README.md` (se muda uso), Swagger (se muda contrato) e `docs/decisions.md` (se houve decisão relevante). Nunca "documentar depois".
- **Testes cobrem a mudança**: regra nova → unit no `service`/`domain`; fluxo/contrato novo → E2E; UI → smoke de parse + verificação registrada; lógica de UI (markdown, link, timeline) → teste onde couber.
- **Git por fase**: trabalhar na **branch da onda** (ver "Branch"). Ao concluir cada **Fase**, `make check` verde → **commit** em PT-BR referenciando a fase. **Push só no fim da onda** (após a Fase F verde), como parte do PR grande para `main`.

**Branch:** esta onda integra o mesmo PR grande que a `mvp-audit1` (decisão do Fino: tudo vira um PR para `main`). Continuar na branch `mvp-audit1` ou criar `prontuario-ux` a partir dela — **confirmar no início da implementação**. Independentemente disso, commits referenciam a fase (`prontuario-ux Fase X`).

**Ordem recomendada:** B (backend edição) → A (WhatsApp/normalização, isolado) → C (Prontuário) → D (calendário) → E (paleta) → F (seed + fechamento). As fases A e E são independentes e podem ser antecipadas se útil.

---

## Fase A — Normalização de telefone + WhatsApp (R3)

- [ ] **A1. `NormalizePhone` no domínio + validação** (dep: —)
  - `domain.NormalizePhone` (só dígitos, DDI 55 quando ausente, preserva DDI existente); helper `onlyDigits`.
  - `Patient.Validate`: após normalização, exigir telefone plausível (12–13 dígitos canônicos), mensagem PT-BR com exemplo.
  - **Aceite:** `"11999990001"`→`"5511999990001"`; `"(11) 99999-0001"`→idem; `"5511999990001"`→inalterado; `"+55 11 3333-4444"`→`"551133334444"`; `"123"`→inválido.
  - **Testes:** unit de `NormalizePhone` (tabela de casos) + `Validate` (válido/ inválido).
  - **Refs:** R3.1 · design §1.

- [ ] **A2. Aplicar normalização no serviço de paciente** (dep: A1)
  - Chamar `domain.NormalizePhone(in.Phone)` em `PatientService.Create`, `Update` e `RegisterFromPortal` (espelhando o padrão de `NormalizeCPF`), antes de `Validate`.
  - **Aceite:** cadastro admin, edição e auto-cadastro do portal persistem o telefone canônico.
  - **Testes:** E2E — cadastrar paciente no admin com telefone mascarado e verificar `phone` canônico na resposta; idem auto-cadastro no portal.
  - **Refs:** R3.1 · design §1.

- [ ] **A3. Link de WhatsApp na UI do admin** (dep: A2)
  - Helper `Wa.link`/`Wa.pretty` em `api.js`; nos 3 pontos de telefone (cabeçalho do detalhe, aba Dados, lista `patientsPanel`), renderizar `<a href="https://wa.me/…" target="_blank" rel="noopener">` com fallback "—" quando sem telefone.
  - **Aceite:** telefone vira link que abre o WhatsApp Web; sem telefone não renderiza link quebrado.
  - **Testes:** smoke de parse dos templates; verificação manual registrada (clique abre `wa.me`).
  - **Refs:** R3.2 · design §4.

---

## Fase B — Edição de sessão (R2.2) — backend

- [ ] **B1. `SessionService.Update`** (dep: —)
  - `SessionUpdateInput{LocationID, Modality, StartsAt, EndsAt}`; `Update(id, in)`: carrega, bloqueia se `IsTerminal` (mensagem PT-BR), aplica campos, `Validate`, `ensureNoConflict` quando `agendada`, persiste via `repo.Update` (já existente).
  - Não altera o paciente (sem `PatientID` no input).
  - **Aceite:** edita horário/local/modalidade de sessão não-terminal; recusa edição de sessão realizada/cancelada/falta.
  - **Testes:** unit — sucesso; estado terminal → validação; fim ≤ início → validação.
  - **Refs:** R2.2 · design §2.

- [ ] **B2. `PUT /v1/admin/sessions/{id}` + Swagger** (dep: B1)
  - Handler `update` (parse RFC3339, 422 em formato inválido), `svc.Update`, audit `AuditActionUpdate`/"session", resposta `sessionView`. Rota no `Register`. Atualizar Swagger.
  - **Aceite:** `PUT` edita e devolve a sessão atualizada; auditado; paciente não muda.
  - **Testes:** E2E — editar sessão agendada (200, campos novos); datas inválidas (422); sessão terminal (422); not found (404).
  - **Refs:** R2.2 · design §2, §8.

---

## Fase C — Prontuário unificado + painel da sessão (R1, R2.1, R2.3, R2.4)

- [ ] **C1. Aba "Prontuário" e timeline** (dep: B2)
  - Substituir abas "Sessões" e "Notas" por "Prontuário"; manter "Anamnese". `buildProntuario()` montando timeline por dia (sessões + notas livres), desc, com registros de sessão aninhados; resumo (total/realizadas/faltas/última). Render Markdown via `Md.toHtml`. Estado vazio acolhedor.
  - **Aceite:** uma aba única mostra o diário por data; notas livres legadas aparecem; registros aparecem sob a sessão correta.
  - **Testes:** smoke de parse dos templates; verificação manual registrada (timeline com sessões + notas).
  - **Refs:** R1.1–R1.4, R2.4 · design §3.1–3.3.

- [ ] **C2. Painel da sessão: edição + registros** (dep: C1)
  - Abrir painel ao clicar numa sessão (no Prontuário). Form de edição → `Api.put('/v1/admin/sessions/'+id, …)`; editor Markdown de registro → `Api.post('/v1/admin/patients/'+pid+'/notes', {session_id:id, content})`; remover registro (confirmação amigável). Edição escondida/desabilitada em estado terminal; registros sempre permitidos. Hint de Google quando `meet_url` e horário alterado.
  - **Aceite:** editar a sessão e adicionar/remover registros vinculados; registro aparece correlacionado no Prontuário.
  - **Testes:** E2E — criar sessão, `PUT` editar, `POST` nota com `session_id`, `GET` notas e conferir `session_id`; smoke de parse.
  - **Refs:** R2.1, R2.3, R2.4 · design §3.4.

---

## Fase D — Calendário semanal fluido (R5)

- [ ] **D1. Dia que cresce com a lotação** (dep: —; integra melhor após B/C)
  - Ajustar `.week-grid`/`.week-day`/`.slot` (CSS) para colunas que crescem em altura (`align-items: start`, sem `min-height` travando), cartões legíveis e alvo ≥ 44px; manter seg–sáb e destaque do dia atual; responsivo em coluna única.
  - **Aceite:** um dia com ~10 sessões fica legível e não deforma os vizinhos; clique no cartão abre o painel da sessão (C2).
  - **Testes:** smoke de parse; verificação manual registrada com seed lotado (F1).
  - **Refs:** R5.1–R5.4 · design §6.

---

## Fase E — Nova paleta (R4)

- [ ] **E1. Trocar tokens de cor para azul royal + lilás** (dep: —)
  - `:root` em `app.css`: `--primary`/strong/soft (azul royal), `--accent`/soft (lilás), `--bg`/`--surface-2` (fundos claros derivados), `--ring` lilás; manter `--ok/--warn/--danger`. Revisar gradientes/cores teal hardcoded (logo/hero).
  - **Aceite:** UI do admin e portal exibem a nova paleta sem resquício de verde/teal fora da semântica de estado; contraste AA preservado.
  - **Testes:** verificação manual de contraste (AA) nos botões primários e texto; registro na task.
  - **Refs:** R4.1–R4.3 · design §5.

---

## Fase F — Seed, fechamento e docs (R3.3 + processo de entrega)

- [ ] **F1. Seed com várias sessões no mesmo dia** (dep: A2)
  - `scripts/seed.sh`: telefones que normalizam para `55…`; ~8–10 sessões num mesmo dia em horários sequenciais + algumas em outros dias (seg–sáb), status variados.
  - **Aceite:** `make stack-up`/`stack-seed` popula um dia lotado para exercitar o calendário (D1) e links de WhatsApp (A3).
  - **Testes:** rodar o seed na stack local; registrar verificação.
  - **Refs:** R3.3, R5.1 · design §7.

- [ ] **F2. Fechamento da onda** (dep: todas)
  - Atualizar `docs/requirements.md` (prontuário unificado, edição de sessão, WhatsApp), `docs/architecture.md` (PUT sessions, normalização de telefone, timeline), `README.md` (se muda uso), Swagger (PUT sessions), `docs/decisions.md` (prontuário unificado; edição de sessão sem reconciliar Google; paleta; calendário fluido vs grade de horas) e `docs/release-notes.md` (entrada da onda, linguagem orientada ao terapeuta).
  - `make check` verde (fmt/vet/build/unit/e2e). Commit final da fase. Push como parte do PR grande para `main` (confirmar com o Fino o momento do push).
  - **Aceite:** docs coerentes com o código; gate verde; release notes publicadas.
  - **Refs:** processo de entrega · requirements "Fora de escopo".

---

## Rastreamento requisito → task

| Requisito | Task(s) |
|---|---|
| R1 Prontuário unificado | C1 |
| R2.1 Abrir sessão | C2, D1 |
| R2.2 Editar sessão (PUT) | B1, B2 |
| R2.3 Registros por sessão | C2 |
| R2.4 Correlação sessão↔registros | C1, C2 |
| R3.1 Normalização de telefone | A1, A2 |
| R3.2 Link WhatsApp | A3 |
| R3.3 Seed | F1 |
| R4 Nova paleta | E1 |
| R5 Calendário fluido | D1, F1 |
| Processo/docs/release | F2 |
