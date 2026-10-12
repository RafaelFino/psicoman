# patient-timeline — Plano de Implementação (Tasks)

Execução incremental. Cada task é demonstrável e testável, apoia-se nas anteriores e integra ao todo (sem código órfão). Refs apontam para [`requirements.md`](./requirements.md), [`design.md`](./design.md) e a fonte durável [`docs/requirements.md`](../../../docs/requirements.md).

Convenções:
- `[ ]` pendente · `[~]` em progresso · `[x]` concluído.
- Cada task só é concluída com **build verde**, **testes verdes** e **demo funcional** (smoke via HTTP e/ou tela).
- Toda mensagem ao usuário em PT-BR; sem `window.prompt/confirm`; WCAG AA nos controles novos.
- Backend imposto no servidor (defense in depth); UI apenas reflete.

**Definição de pronto (obrigatória em toda task — ver [`.kiro/steering/psicoman-processo-entrega.md`](../../steering/psicoman-processo-entrega.md)):**
- **Docs junto do código**: toda mudança de comportamento atualiza, no mesmo ciclo, `docs/requirements.md` (o quê), `docs/architecture.md` (como), `README.md` (se muda uso), Swagger (se muda contrato) e `docs/decisions.md` (se houve decisão relevante). Nunca "documentar depois".
- **Testes cobrem a mudança**: regra nova → unit no `service`; fluxo/contrato novo → E2E; segurança/acesso → E2E permitido **e** negado; UI → smoke de parse + verificação registrada; lógica de UI (dinheiro, markdown, grade de calendário, ordenação de feed) → teste onde couber.
- **Git por fase**: trabalhar na **branch da onda** (`patient-timeline`). Ao concluir cada **Fase**, `make check` verde → **commit** em PT-BR referenciando a fase. **Push só no fim da onda** (após a Fase I verde).

**Ordem das fases (dependências):** A (faturamento) → B (status da sessão: base para history/métricas/relatório/feed) → C (history backend) → D (timeline UI) → E (calendário) → F (métricas do paciente + relatório por local) → G (feed do dia do terapeuta) → H (prontuário feed + título) → I (fechamento).

---

## Fase A — Faturamento: dia de vencimento e débito automático (R1)

- [x] **A1. Plano ganha dia de vencimento** (dep: —)
  - `domain.Plan.DueDay int` (1–28; 0 = não aplicável); `Validate()` aceita 0 ou 1–28.
  - Migration `0003_plan_due_day.sql`: `ALTER TABLE plan ADD COLUMN due_day INTEGER NOT NULL DEFAULT 1;` + `UPDATE plan SET due_day = 5 WHERE type IN ('plano_fechado_mensal','plano_fechado_trimestral');`
  - Repositório SQLite (`repository/sqlite/plan.go`): `due_day` em insert/select/update.
  - **Aceite:** boot aplica migration em base com e sem dados; planos fechados pré-existentes ficam com `due_day = 5`.
  - **Testes:** unit de validação; migration em base com/sem dados.
  - **Refs:** R1.5 · D1.1.

- [x] **A2. UI de plano expõe o dia de vencimento** (dep: A1)
  - Handler/serviço de plano (`internal/api/admin/plan.go`): aceita e retorna `due_day`.
  - UI (`admin_app.html`, aba Plano): seletor 1–28 visível só quando o tipo é fechado (mensal/trimestral).
  - Swagger atualizado para o campo `due_day`.
  - **Aceite:** criar/editar plano fechado grava `due_day`; plano por sessão/social ignora o campo.
  - **Testes:** E2E de criação/edição com `due_day`; smoke de parse do template.
  - **Refs:** R1.5 · D1.1.

- [x] **A3. Débito automático na sessão realizada (opt-out)** (dep: A1)
  - `BillingService.OnSessionFinished`: geração por padrão para plano por-sessão; `Bill == false` = opt-out explícito. `SessionService.Finish` marca `Bill = true` por padrão salvo desmarcação explícita.
  - Mantém: plano vigente, pula social/fechado, idempotência `session:<id>`, audit `AuditActionDebtGenerate`; sem plano ⇒ skip observável (`debt_skip_no_plan`).
  - **Aceite:** finalizar sessão de plano por-sessão gera 1 débito; reprocessar não duplica; social/fechado não geram; sem plano registra skip.
  - **Testes:** unit `billing` (fakes); E2E `POST /sessions/{id}/finish` → `GET /debts?patient_id=`.
  - **Refs:** R1.1–R1.4 · D1.2.

- [x] **A4. Ciclo por vencimento** (dep: A1)
  - `BillingService.CloseCycles`: só gera o débito do período quando `ref.Day() >= plan.DueDay`; `due_date` = vencimento real do período; idempotência `cycle:<plan_id>:<período>` inalterada; trimestral dispara pelo `due_day` do 1º mês.
  - Cron (`internal/app/cron.go`) inalterado (gatilho diário).
  - **Aceite:** antes do vencimento ⇒ 0 débitos; a partir do vencimento ⇒ 1; rodar vários dias depois ⇒ continua 1.
  - **Testes:** unit `billing` com `clock` em datas antes/depois do vencimento; E2E `POST /billing/close-cycles`.
  - **Refs:** R1.2, R1.5 · D1.3.

- [x] **Fechamento Fase A:** `docs/decisions.md` (opt-out do `Bill`, dia de vencimento no plano, due_date real), `docs/requirements.md`/`docs/architecture.md` atualizados; Swagger (`due_day`); `make check` verde → commit "patient-timeline Fase A: débito automático por sessão e ciclo por vencimento".

---

## Fase B — Controle de status da sessão (R6 base: domínio, persistência, serviço)

- [x] **B1. Máquina de estados + metadados no domínio** (dep: —)
  - `domain.Session`: nova const `SessionConfirmada`; atualizar `transitions` (`agendada → confirmada|realizada|cancelada|falta`; `confirmada → realizada|cancelada|falta`); `ValidStatus` reconhece `confirmada`.
  - Novos campos: `ConfirmedAt`, `CanceledBy`, `CancelReason`, `CanceledAt`; consts `CancelByPatient`/`CancelByTherapist`; helper `ValidCancelBy`.
  - Função pura `SessionCategory(sess)` → categoria de relatório (realizadas, faltas, cancelada-paciente, cancelada-terapeuta, cancelada-sem-autor, agendadas/confirmadas, solicitadas).
  - **Aceite:** transições novas aceitas; inválidas rejeitadas; `SessionCategory` cobre todos os casos.
  - **Testes:** unit domínio (transições, `ValidCancelBy`, `SessionCategory`).
  - **Refs:** R6.1–R6.2, R6.4 · D6.1, D6.4.

- [x] **B2. Migration de rebuild + persistência** (dep: B1)
  - Migration `0004_session_status_meta.sql`: adiciona `confirmed_at`/`canceled_by`/`cancel_reason`/`canceled_at` e **recria a tabela `session`** (table rebuild) para o CHECK de status aceitar `confirmada`, preservando dados, FKs e índices.
  - Repositório SQLite (`repository/sqlite/session.go`): 4 colunas novas em insert/select/update; zero-time ↔ NULL.
  - **Aceite:** boot aplica em base com e sem dados; dados preservados após rebuild; `confirmada` aceito; metadados nulos nos pré-existentes.
  - **Testes:** migration em base com/sem dados; round-trip de persistência dos metadados.
  - **Refs:** R6.3 · D6.2.

- [x] **B3. Serviço: confirmar e cancelar com autor/motivo** (dep: B2)
  - `SessionService.Confirm` (`agendada → confirmada`, grava `confirmed_at`, audit `session.confirm`); `Cancel(ctx, id, by, reason)` exige `by` válido, grava metadados, audit `session.cancel` (metadata `{canceled_by}`, sem motivo em metadata); `Finish`/`MarkNoShow` aceitam origem `confirmada`.
  - Handlers (`internal/api/admin/session.go`): `POST /sessions/{id}/confirm`; `POST /sessions/{id}/cancel` com corpo `{canceled_by, cancel_reason}`; `sessionView` expõe os campos novos. Swagger atualizado.
  - UI mínima: ação "Confirmar" e cancelamento com seletor de autor + motivo no painel da sessão; `Fmt.sessionStatus` e `statusChip` ganham `confirmada`.
  - **Aceite:** confirmar grava `confirmed_at`; cancelar exige autor (negado sem autor) e grava metadados; audit sem conteúdo clínico.
  - **Testes:** unit serviço; E2E confirm; E2E cancel com e sem autor (negado sem autor); smoke de parse.
  - **Refs:** R6.1–R6.2 · D6.3, D6.5.

- [x] **Fechamento Fase B:** `docs/decisions.md` (Opção B para status; rebuild da `session` na 0004), `docs/architecture.md` (metadados de sessão, categorização), `docs/requirements.md`, Swagger; `make check` verde → commit "patient-timeline Fase B: controle de status da sessão (confirmação, autor de cancelamento)".

---

## Fase C — Endpoint de histórico do paciente (R4 backend)

- [x] **C1. Rastreabilidade de remarcação** (dep: B3)
  - `SessionService.Update`: quando altera `StartsAt`/`EndsAt` de sessão `agendada`/`confirmada`, grava audit `session.reschedule` com metadata `{session_id, from, to}` (datas, sem conteúdo clínico).
  - **Aceite:** remarcar uma sessão gera entrada de audit consultável.
  - **Testes:** unit do serviço; E2E `PUT /sessions/{id}` com nova data → entrada no audit.
  - **Refs:** R4.3 · D4.3.

- [x] **C2. Serviço de composição do histórico** (dep: C1, A3)
  - Novo serviço/método que compõe o feed a partir de: cadastro/aprovação, sessões (com status/autor de cancelamento), remarcações (audit), registros/notas, anamnese, débitos, pagamentos, arquivos. Itens `{tipo, data, titulo, resumo, ref}`, ordenados desc. Leitura de conteúdo clínico gera `AuditActionRead`.
  - **Aceite:** devolve todos os tipos na ordem correta, sem conteúdo clínico em metadata de audit.
  - **Testes:** unit com fakes (ordenação, presença de cada tipo, remarcação/cancelamento do audit).
  - **Refs:** R4.1–R4.2, R4.5 · D4.1, D4.4, D6.5.

- [x] **C3. Endpoint `GET /patients/{id}/history`** (dep: C2)
  - Handler admin fino; `historyView` serializa; envelope padrão. Swagger atualizado.
  - **Aceite:** endpoint devolve o feed ordenado para um paciente com histórico variado.
  - **Testes:** E2E — paciente → sessão → confirmar → finalizar (débito) → pagar → anexar arquivo → remarcar → cancelar outra; `GET /patients/{id}/history` traz todos os eventos na ordem esperada.
  - **Refs:** R4.1–R4.2 · D4.4.

- [x] **Fechamento Fase C:** `docs/architecture.md`/`docs/requirements.md`, Swagger, `docs/decisions.md` (composição no servidor); `make check` verde → commit "patient-timeline Fase C: histórico do paciente e rastreabilidade de remarcação".

---

## Fase D — Timeline como tela base do paciente (R4 UI)

- [x] **D1. Helpers de formatação de feed** (dep: —)
  - `Fmt.relative(iso)` em `internal/web/static/api.js` (datas relativas "hoje", "há 2 dias" + absoluta no title).
  - **Aceite:** helper devolve rótulos coerentes para hoje/ontem/semana/data antiga.
  - **Testes:** teste do helper onde couber; senão verificação manual registrada.
  - **Refs:** R3.3, R4.1 · D3.1.

- [x] **D2. Visão timeline no `patientDetail()`** (dep: C3, D1)
  - Nova aba default `tab = 'timeline'`; consome `GET /patients/{id}/history`; cards por tipo (ícone/cor), ação ao clicar (abrir sessão/débito/arquivo), filtro "Tudo | Clínico | Financeiro | Agenda | Arquivos"; eventos de cancelamento mostram autor/motivo.
  - **Aceite:** timeline é a visão inicial; filtros funcionam; "Tudo" é padrão; cliques navegam.
  - **Testes:** smoke de parse; verificação manual registrada.
  - **Refs:** R4.1, R4.4 · D4.2, D6.5.

- [x] **Fechamento Fase D:** `README.md` (tela base = timeline), `docs/requirements.md`; `make check` verde → commit "patient-timeline Fase D: timeline como tela base do paciente".

---

## Fase E — Calendário mensal do paciente (R2)

- [x] **E1. Grade mensal no `patientDetail()`** (dep: B3)
  - Nova visão "Calendário"; carrega `GET /sessions?patient_id=&from=&to=` do mês visível; monta grade 7×n agrupando por dia (fuso local); chips de status por cor (inclui `confirmada`); navegação ‹/›/Hoje; clique abre `openProntuarioSession`.
  - **Aceite:** calendário mostra sessões passadas e futuras; navegação de meses funciona; clique abre o registro da sessão.
  - **Testes:** smoke de parse; lógica de grade testada onde couber; verificação manual registrada.
  - **Refs:** R2.1–R2.2 · D2.

- [x] **Fechamento Fase E:** `docs/requirements.md`/`README.md`; `make check` verde → commit "patient-timeline Fase E: calendário mensal do paciente".

---

## Fase F — Métricas do paciente e relatório por local (R6.4–R6.5, R7)

- [ ] **F1. Motor de agregação + métricas do paciente** (dep: B3)
  - Serviço de relatório com agregador por `SessionCategory` (+ faturado das realizadas), agrupando por `starts_at` no fuso local. `PatientSessionMetrics(ctx, id, from, to)` com taxas zero-safe (comparecimento, cancelamento).
  - Endpoint `GET /v1/admin/patients/{id}/session-metrics?from=&to=` (default = histórico completo); `metricsView`. Swagger atualizado.
  - UI: bloco de KPIs no detalhe do paciente (reusa `.kpis`/`.kpi`), rótulos PT-BR; taxas em %.
  - **Aceite:** métricas conferem com sessões de todos os status; taxas corretas e zero-safe.
  - **Testes:** unit do agregador; E2E com sessões variadas; smoke de parse.
  - **Refs:** R6.4–R6.5 · D7.1–D7.2.

- [ ] **F2. Relatório por local** (dep: F1)
  - Repositório `sessions.ListByLocationRange`; serviço `LocationSessionReport(ctx, locationID, from, to)` (counts por categoria + faturado das realizadas).
  - Endpoint `GET /v1/admin/locations/{id}/session-report?from=&to=` (default = mês atual); Swagger atualizado.
  - UI: seção no admin (junto de Locais ou Custos e ROI), seletor de local + navegação de período, tabela de categorias + total + faturado; padrão visual de "Custos e ROI".
  - **Aceite:** relatório mensal por local confere contagens, total e faturado; navegação de período funciona.
  - **Testes:** unit do agregador por local; E2E; smoke de parse.
  - **Refs:** R7.1–R7.4 · D7.1, D7.3–D7.4.

- [ ] **Fechamento Fase F:** `docs/requirements.md`/`docs/architecture.md`/`README.md`, Swagger, `docs/decisions.md` (agrupamento por `starts_at`, CSV fora de escopo); `make check` verde → commit "patient-timeline Fase F: métricas do paciente e relatório por local".

---

## Fase G — Feed do dia do terapeuta (R8)

- [ ] **G1. Backend: agregação do feed do dia** (dep: C2, F1)
  - `DayFeedService` (ou método no serviço de relatório) que compõe eventos transversais a todos os pacientes num intervalo (`window=hoje|proximos7|recentes`): sessões por `starts_at`, pagamentos, débitos (criação + vencimentos próximos), arquivos, aprovações, pedidos pendentes. Reaproveita a normalização de eventos (D4) e o motor de agregação (F1) para o bloco "Resumo do dia".
  - Endpoint `GET /v1/admin/day-feed?window=`; `dayFeedView` → `{resumo, eventos[]}`. Leitura clínica (se houver) gera `AuditActionRead`. Swagger atualizado.
  - **Aceite:** feed devolve eventos de todos os tipos no intervalo + resumo correto; janelas mudam o conjunto; sem conteúdo clínico em metadata.
  - **Testes:** unit do agregador (separação passado/futuro, resumo); E2E `GET /day-feed` nas três janelas.
  - **Refs:** R8.1–R8.2, R8.5 · D8.1–D8.2, D8.4.

- [ ] **G2. UI: feed do dia como visão inicial do admin** (dep: G1)
  - `dayFeedPanel()` em `admin_app.html`; nova visão `#hoje` como entrada padrão do app, agenda (grade) acessível pelo menu. Agrupamento por faixa ("Agora", "Mais tarde hoje", "Amanhã", "Já passou"); cards didáticos (paciente + data relativa/absoluta + tipo/status/valor), clique abre paciente/sessão. Bloco "Resumo do dia" **colapsado por padrão** (`aria-expanded`, teclado), eventos individuais visíveis.
  - **Aceite:** ao abrir o admin, o feed do dia é a visão inicial; resumo nasce colapsado e expande; cards navegam; agenda continua acessível.
  - **Testes:** smoke de parse; verificação manual (janelas, colapso do resumo, navegação).
  - **Refs:** R8.1, R8.3–R8.4 · D8.3.

- [ ] **Fechamento Fase G:** `docs/requirements.md`/`docs/architecture.md`/`README.md`, Swagger; `make check` verde → commit "patient-timeline Fase G: feed do dia do terapeuta".

---

## Fase H — Feed de prontuário e título (R3, R5)

- [ ] **H1. Prontuário como feed cronológico reverso** (dep: D1)
  - Reordenar o feed clínico desc (mais recente no topo); Anamnese como card fundacional destacado no topo ("Ficha inicial") e marco no fim; composição rápida de nota livre no topo; preservar editar/salvar anamnese e add/remove nota.
  - **Aceite:** feed clínico ordenado do mais recente ao mais antigo; anamnese destacada; adicionar nota pelo topo funciona; nenhuma regressão.
  - **Testes:** ordenação testada onde couber; smoke de parse; verificação manual registrada.
  - **Refs:** R3.1–R3.3 · D3.

- [ ] **H2. Título com a marca "Psicoman"** (dep: —)
  - `layout.html` (`head`): `<title>Psicoman · {{.Title}}</title>`; topbar inalterada (usa só `{{.Title}}`), sem duplicar a marca.
  - **Aceite:** aba do navegador mostra "Psicoman · …" em admin e portal, todas as páginas.
  - **Testes:** smoke de parse; verificação manual do `<title>` registrada.
  - **Refs:** R5.1 · D5.

- [ ] **Fechamento Fase H:** `docs/requirements.md`/`README.md`; `make check` verde → commit "patient-timeline Fase H: feed de prontuário e título com a marca".

---

## Fase I — Fechamento da onda

- [ ] **I1. Revisão integrada e release notes** (dep: A–H)
  - Conferir Definição de Pronto em todas as fases; `docs/release-notes.md` com a entrada da onda (Novidades: débito automático por sessão, ciclo por vencimento, controle completo de status da sessão, timeline do paciente, calendário mensal, métricas de sessão no prontuário, relatório por local, feed do dia do terapeuta, feed de prontuário; Melhorias: título com marca; Notas de operação: migrations `0003`/`0004`, rebuild da `session`, cron de ciclo).
  - `make check` verde na branch inteira.
  - **Push da branch `patient-timeline`** (fim da onda) e abertura de PR para `main`.
  - **Refs:** processo de entrega.
