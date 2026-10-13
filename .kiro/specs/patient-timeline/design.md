# patient-timeline — Design

Como implementar os requisitos de [`requirements.md`](./requirements.md), respeitando a arquitetura vigente ([`docs/architecture.md`](../../../docs/architecture.md)): Go puro, SQLite, SSR com `html/template` + Alpine.js (sem SPA), envelope de API padrão, domínio magro + services, IDs ULID, dinheiro em centavos, timestamps ISO-8601 (`America/Sao_Paulo`).

Princípios desta onda:
- **Derivar, não duplicar**: a timeline é composta a partir das fontes existentes (sessões, notas, anamnese, débitos, pagamentos, arquivos, aprovação, audit). Nada de tabela nova de "eventos" se puder ser derivada.
- **Reaproveitar o que existe**: envelope JSON, `panel()`/`Fmt`/`Md.toHtml`/`Wa`/`Money` (`internal/web/static/api.js`), design system CSS, filtros já existentes de `GET /sessions`.
- **Backend como fonte**: idempotência de débito e enforcement de auditoria no servidor; a UI reflete.
- **Nada de código órfão**: cada peça nova de backend é exercida por uma tela; cada tela consome endpoints reais.

Arquivos-âncora (do mapeamento do código atual):
- Domínio: `internal/domain/{plan,debt,session,prontuario,patient,audit}.go`.
- Serviços: `internal/service/{billing,session,appointment,prontuario}.go`.
- Billing: `internal/service/billing.go` (`OnSessionFinished`, `CloseCycles`, `billingPeriodFor`).
- Cron: `internal/app/cron.go`.
- Migrations: `internal/migration/sql/` (próxima: `0003_*`).
- UI admin: `internal/web/templates/admin_app.html` (bloco `patientDetail()`), `internal/web/web.go` (títulos), `internal/web/templates/layout.html`.

---

## D1. Débito automático por sessão + ciclo por vencimento (R1)

### D1.1 Plano: dia de vencimento
- Acrescentar `DueDay int` ao `domain.Plan` (1–28). `Validate()` do plano aceita só 0 (não aplicável a planos por sessão/social) ou 1–28.
- Migration **`0003_plan_due_day.sql`** (aditiva, idempotente no boot):
  - `ALTER TABLE plan ADD COLUMN due_day INTEGER NOT NULL DEFAULT 1;`
  - `UPDATE plan SET due_day = 5 WHERE type IN ('plano_fechado_mensal','plano_fechado_trimestral');` — padrão coerente (dia 5) para planos fechados pré-existentes; demais tipos ficam com 1 (irrelevante, não geram ciclo).
- Repositório SQLite (`repository/sqlite/plan.go`): incluir `due_day` em insert/select/update.
- Serviço de plano e handler admin (`internal/api/admin/plan.go`): aceitar/retornar `due_day`; a UI de plano ganha o campo (seletor 1–28) só quando o tipo é fechado.

### D1.2 Débito por sessão (ajuste de `OnSessionFinished`)
- Manter `OnSessionFinished` como hook de `SessionService.Finish`.
- Mudança de semântica (R1.3): o padrão passa a ser **gerar** o débito para sessão `realizada` de plano por-sessão; `Bill == false` vira **opt-out explícito**.
  - Hoje: `if !sess.Bill { return nil }` (não gera sem flag).
  - Novo: a geração ocorre por padrão; `Bill == false` suprime deliberadamente. Como `Bill` default no schema é `0`, o `SessionService.Finish` DEVE passar a marcar `Bill = true` por padrão ao finalizar (a menos que o terapeuta tenha explicitamente desmarcado na tela de finalização). Decisão registrada em `docs/decisions.md`.
- Mantém: busca de plano vigente (`GetActiveByPatient`), pula social e plano fechado, idempotência `session:<id>` via `InsertIfAbsent`, audit `AuditActionDebtGenerate`.
- QUANDO não há plano vigente: não gera (sem base) e registra audit/log observável (`debt_skip_no_plan` em metadata, sem conteúdo clínico).

### D1.3 Débito por ciclo (ajuste de `CloseCycles`)
- `billingPeriodFor` permanece devolvendo `YYYY-MM` (mensal) / `YYYY-Q` (trimestral).
- Nova condição de gatilho: o débito do período só é criado quando `ref.Day() >= plan.DueDay`.
  - Mensal: período = mês de `ref`; dispara a partir do dia `due_day` do mês.
  - Trimestral: período = trimestre de `ref`; dispara a partir do dia `due_day` do **primeiro mês** do trimestre (mantém uma cobrança por trimestre; idempotência por `YYYY-Q` garante unicidade).
- `due_date` do débito = data real do vencimento no período (ano/mês do período + `due_day`), substituindo `now.AddDate(0,0,7)`.
- Idempotência inalterada: `CycleIdempotencyKey(plan.ID, period)` + índice único `ux_debt_idempotency`. Rodar o cron todo dia após o vencimento não duplica.
- Cron (`internal/app/cron.go`) continua chamando `CloseCycles(ctx, clock.Now())` diariamente; nenhuma mudança de agendamento.

### D1.4 Testes
- Unit `billing`: fakes de `plans`/`debts`/`clock`.
  - Por sessão: realizada + plano por-sessão ⇒ 1 débito; reprocessar ⇒ 0 novos (idempotência); social/fechado ⇒ 0; sem plano ⇒ 0 + skip observável; `Bill=false` opt-out ⇒ 0.
  - Por ciclo: `ref.Day() < due_day` ⇒ 0; `ref.Day() >= due_day` ⇒ 1; rodar vários dias após ⇒ continua 1 (idempotência); `due_date` = vencimento real.
- E2E: finalizar sessão de paciente com plano por-sessão ⇒ débito aparece em `GET /debts?patient_id=`; disparar `POST /billing/close-cycles` antes/depois do vencimento.

---

## D2. Calendário mensal do paciente (R2)

### D2.1 Dados (sem backend novo)
- Reusar `GET /v1/admin/sessions?patient_id=<id>&from=<RFC3339>&to=<RFC3339>` já existente.
- O cliente carrega o intervalo do mês visível (primeiro ao último dia do mês, com folga para dias de borda da grade) e monta a grade localmente, agrupando por dia no fuso local.

### D2.2 UI (Alpine, dentro de `patientDetail()`)
- Nova aba/visão "Calendário" na `nav.tabs` do detalhe do paciente (`admin_app.html`), ou subcomponente da timeline.
- Estado: `calRef` (data de referência do mês), `calSessions` (sessões do intervalo), map `dia(YYYY-MM-DD) -> [sessões]`.
- Grade 7×(5–6): cabeçalho seg–dom (PT-BR), células com número do dia; dias fora do mês esmaecidos.
- Célula com sessão(ões): chip de status por cor (reusar `statusChip`): `realizada=ok`, `agendada/solicitada=info`, `falta=warn`, `cancelada=danger`. Badge com a contagem quando > 1.
- Navegação: botões ‹ / › (mês anterior/próximo) e "Hoje"; ao mudar o mês, recarrega o intervalo.
- Clique no dia/sessão: reusa `openProntuarioSession(sessionId)` (já existe) para abrir o registro da sessão.
- Acessibilidade: grade com `role="grid"`, dias focáveis, rótulos `aria-label` com data + status; WCAG AA de contraste nos chips (já no design system).

### D2.3 Testes
- Smoke de parse do template (já há harness de parse em `internal/web`).
- Lógica de montagem da grade (agrupamento por dia, bordas de mês): se extrair helper JS puro, teste onde couber; caso contrário, verificação manual registrada na task.

---

## D3. Feed de prontuário (R3)

### D3.1 Reordenação
- O `patientDetail()` já monta `timeline()` por dia (sessões + notas). Ajustar para **ordem desc global** (mais recente no topo) e inserir a **Anamnese** como item fundacional.
- Modelo de item do feed clínico:
  - `anamnese` (quando há conteúdo): data = `updated_at` (ou `created_at`); destaque visual (marco fundacional, ícone/borda distinta); sempre renderizada no fim da ordenação como "início da história", mas também acessível no topo quando foi atualizada recentemente — **decisão**: a Anamnese aparece como um card fixo destacado no topo do feed clínico ("Ficha inicial") E como marco no fim da linha do tempo; evita ambiguidade de ordenação.
  - `registro_sessao` (nota com `session_id`): data = `created_at`; mostra contexto da sessão (data/modalidade/status via `Fmt.short` + `sessionStatus`).
  - `nota_livre` (nota sem `session_id`): data = `created_at`.
- Conteúdo renderizado com `Md.toHtml`. Datas: relativa + absoluta no `title` (novo helper `Fmt.relative(iso)` em `api.js`).

### D3.2 Composição rápida
- Campo de composição no topo ("Registrar o que aconteceu…") que chama o `addNote()` já existente (nota livre). Reaproveita `mdField`/preview.

### D3.3 Preservação
- Manter editar/salvar anamnese, adicionar/remover nota, painel de sessão (`openSession`). Nenhuma regressão de funcionalidade; só reorganização visual + ordenação.

### D3.4 Testes
- Lógica de ordenação desc e posição da anamnese: teste do helper se extraído; senão verificação manual registrada.
- Auditoria de leitura clínica permanece (coberta pelos handlers de prontuário).

---

## D4. Timeline unificada (R4)

### D4.1 Estratégia: derivar no cliente a partir de endpoints existentes
- O `patientDetail().bootstrap()` já carrega sessões, débitos, arquivos e planos; adiciona pagamentos e notas/anamnese e o estado de aprovação (já em `patient`).
- Novo método `historyFeed()` em `patientDetail()` que funde todas as fontes num array de eventos normalizados:

```
{ tipo, data (ISO), icone, cor, titulo, resumo, ref: {kind, id}, acao? }
```

Tipos e mapeamento de fonte:
| tipo            | fonte                                   | data            | ação ao clicar            |
|-----------------|-----------------------------------------|-----------------|---------------------------|
| `cadastro`      | `patient.created_at`                    | created_at      | —                         |
| `aprovacao`     | `patient.approval_status` + audit       | audit.created_at| —                         |
| `sessao`        | `sessions[]`                            | starts_at       | abrir prontuário da sessão|
| `remarcacao`    | audit `session.reschedule` (D4.3)       | audit.created_at| abrir sessão              |
| `registro`      | `notes[]` com `session_id`              | created_at      | abrir sessão              |
| `nota_livre`    | `notes[]` sem `session_id`              | created_at      | —                         |
| `anamnese`      | `anamnesis`                             | updated_at      | abrir aba Anamnese        |
| `debito`        | `debts[]`                               | created_at      | abrir débito              |
| `pagamento`     | `payments[]` (por débito)               | paid_at         | abrir débito              |
| `arquivo`       | `files[]`                               | created_at      | download                  |

- Ordenação desc por `data`. Agrupamento visual por mês/dia opcional (consistente com o feed clínico).
- Ícones/cores reusam o sprite (`{{template "icon" ...}}`) e `statusChip`/`debtStatus`.

### D4.2 UI
- Timeline vira a **visão base** (aba default `tab = 'timeline'`) ao abrir o paciente; as abas atuais seguem acessíveis.
- Filtro por categoria: chips "Tudo | Clínico | Financeiro | Agenda | Arquivos" que filtram o array por conjunto de `tipo`. "Tudo" é o padrão.
- Cada evento é um card compacto (ícone + título + resumo + data relativa/absoluta); clique dispara a ação da tabela acima.

### D4.3 Rastreabilidade de remarcação (único backend novo além do plano)
- Hoje `SessionService.Update` edita data/horário sem registrar de forma consultável e não reconcilia o Google (ver `docs/decisions.md 2026-10-12`).
- Mínimo necessário: quando `Update` altera `StartsAt`/`EndsAt` de uma sessão `agendada`, gravar **audit log** `AuditActionUpdate` (ou nova ação `session.reschedule`) com metadata `{session_id, from, to}` (datas, **sem** conteúdo clínico — coerente com `§4.1`).
- A timeline consome a remarcação via endpoint de auditoria filtrado por entidade/sessões do paciente. Decisão: expor leitura de audit por paciente para a timeline **ou** derivar a entrada de remarcação de um campo adicional na sessão.
  - **Escolha:** registrar no audit log (já existe infra e tela de Auditoria) e, para a timeline, adicionar um endpoint de leitura `GET /v1/admin/patients/{id}/history` que o backend compõe (sessões/débitos/pagamentos/notas/arquivos/remarcações) — **ver D4.4**.

### D4.4 Decisão de composição: cliente vs servidor
- Duas opções:
  - **(A) Compor no cliente** a partir dos endpoints já existentes + um endpoint de audit por paciente para remarcações. Menos backend, mais lógica JS; já temos quase tudo carregado no `bootstrap()`.
  - **(B) Endpoint servidor `GET /patients/{id}/history`** que devolve o feed já normalizado e ordenado. Centraliza a regra, facilita auditoria e testes E2E de contrato, e evita múltiplas chamadas.
- **Escolha: (B)** para a timeline unificada, pela testabilidade (E2E de contrato) e por já respeitar a auditoria de leitura clínica no servidor. O feed clínico (D3) e o calendário (D2) permanecem client-side sobre endpoints existentes. O endpoint `history` agrega: cadastro/aprovação, sessões, remarcações (do audit), registros/notas, anamnese, débitos, pagamentos, arquivos — cada item com `tipo/data/titulo/resumo/ref`. Leitura de conteúdo clínico gera `AuditActionRead`.
- `historyView` (handler admin) devolve itens ordenados desc; a UI só renderiza e filtra.

### D4.5 Testes
- Unit do serviço de composição do histórico (fakes dos repositórios): ordenação desc, presença de cada tipo, remarcação derivada do audit, ausência de conteúdo clínico em metadata.
- E2E: criar paciente → sessão → finalizar (gera débito) → pagar → anexar arquivo → remarcar; `GET /patients/{id}/history` traz todos os eventos na ordem esperada.
- Smoke de parse do template da timeline.

---

## D5. Título com a marca "Psicoman" (R5)

### D5.1 Mudança
- Em `internal/web/web.go`, prefixar a marca em todos os títulos:
  - `index` admin: `Psicoman · Administração`.
  - `index` portal: `Psicoman · Portal do Paciente`.
  - `adminApp`: `Psicoman · Painel`.
  - `portalApp`: `Psicoman · Minha área`.
- Alternativa equivalente: manter os `Title` de contexto e prefixar no template `head` (`<title>Psicoman · {{.Title}}</title>`). **Escolha:** prefixar no `web.go` para manter o template agnóstico e os títulos explícitos no código.
- A topbar (`layout.html`) já mostra `Psicoman <span class="sub">{{.Title}}</span>`; com o prefixo no `Title`, para evitar "Psicoman · Psicoman · Painel" na topbar, ajustar o `pageData` para carregar `Title` (contexto puro, ex.: "Painel") e derivar o `<title>` do navegador com a marca. **Refino:** adicionar `pageData.Brand` ou computar o `<title>` no template como `Psicoman · {{.Title}}` e deixar a topbar usando só `{{.Title}}`. Decisão final: computar no template `head` (`<title>Psicoman · {{.Title}}</title>`) e **não** alterar as strings de `web.go`, mantendo a topbar como está. Isso satisfaz R5 sem duplicar a marca na topbar.

### D5.2 Testes
- Smoke de parse do template; verificação manual do `<title>` renderizado nas páginas admin/portal registrada na task.

---

## D6. Status da sessão: confirmação e metadados de cancelamento (R6)

### D6.1 Máquina de estados (domínio)
- `internal/domain/session.go`: nova constante `SessionConfirmada = "confirmada"`.
- Atualizar o mapa `transitions`:
  - `solicitada → {agendada, cancelada}`
  - `agendada → {confirmada, realizada, cancelada, falta}`
  - `confirmada → {realizada, cancelada, falta}`
  - `realizada | cancelada | falta` → terminais (mapa vazio).
- `IsTerminal()` inalterado (realizada/cancelada/falta). `ValidStatus` passa a reconhecer `confirmada`.
- Novos campos na struct `Session` (metadados ortogonais, Opção B):
  - `ConfirmedAt time.Time` (zero = não confirmada).
  - `CanceledBy string` (`paciente` | `terapeuta`; vazio quando não cancelada).
  - `CancelReason string` (texto livre PT-BR, opcional).
  - `CanceledAt time.Time` (zero = não cancelada).
- Constantes de autor: `CancelByPatient = "paciente"`, `CancelByTherapist = "terapeuta"`; helper `ValidCancelBy(s string) bool`.
- `Validate()`: aceita `confirmada`; não impõe os metadados (preenchidos pelas transições específicas).

### D6.2 Persistência
- Migration **`0004_session_status_meta.sql`** (aditiva, idempotente no boot):
  - `ALTER TABLE session ADD COLUMN confirmed_at TEXT;`
  - `ALTER TABLE session ADD COLUMN canceled_by TEXT;`
  - `ALTER TABLE session ADD COLUMN cancel_reason TEXT;`
  - `ALTER TABLE session ADD COLUMN canceled_at TEXT;`
  - **CHECK de status:** SQLite não altera `CHECK` por `ALTER`. Como o schema inicial define `status ... CHECK (status IN (...))`, incluir `confirmada` exige recriar a tabela (table rebuild: criar `session_new` com o CHECK atualizado, `INSERT INTO session_new SELECT ...` com as novas colunas nulas, drop/rename, recriar índices). A migration DEVE fazer esse rebuild preservando dados e FKs. **Decisão** registrada em `docs/decisions.md`: rebuild controlado na migration 0004.
  - Registros pré-existentes: metadados nulos; status inalterado.
- Repositório SQLite (`repository/sqlite/session.go`): incluir as 4 colunas em insert/select/update; mapear zero-time ↔ NULL.

### D6.3 Serviço
- `SessionService`:
  - `Confirm(ctx, id)`: `agendada → confirmada`, grava `confirmed_at = clock.Now()`; audit `AuditActionUpdate`/`session.confirm`.
  - `Cancel(ctx, id, by, reason)`: exige `by` válido (`ValidCancelBy`); grava `canceled_by`, `cancel_reason`, `canceled_at`; transição para `cancelada`; audit `session.cancel` com metadata `{canceled_by}` (sem motivo clínico em metadata — motivo fica só no registro da sessão).
  - `Finish`/`MarkNoShow` inalterados salvo aceitar origem `confirmada` além de `agendada`.
- Handlers admin (`internal/api/admin/session.go`): `POST /sessions/{id}/confirm`; `POST /sessions/{id}/cancel` passa a aceitar corpo `{canceled_by, cancel_reason}`.
- Swagger atualizado (novo endpoint confirm; corpo do cancel; campos novos no `sessionView`).

### D6.4 Status derivado (categoria de relatório)
- Função pura de categorização (domínio ou pacote de relatório), reutilizada por R6.4 e R7:
  - `realizada` → **Realizadas**
  - `falta` → **Faltas**
  - `cancelada` + `canceled_by = paciente` → **Canceladas pelo paciente**
  - `cancelada` + `canceled_by = terapeuta` → **Canceladas pelo terapeuta**
  - `cancelada` + autor vazio (legado) → **Canceladas (sem autor)**
  - `agendada` | `confirmada` → **Agendadas/Confirmadas**
  - `solicitada` → **Solicitadas**
- Entregue como `SessionCategory(sess)` → enum de categoria; os relatórios só contam categorias.

### D6.5 Impacto na timeline (D4) e no calendário (D2)
- A remarcação (D4.3) e o cancelamento com autor passam a enriquecer os eventos: o evento `sessao`/`remarcacao`/`cancelamento` na timeline exibe o autor do cancelamento e o motivo quando houver.
- `statusChip` no calendário ganha `confirmada` → cor `info` (ou um tom distinto de `agendada`).
- `Fmt.sessionStatus` em `api.js` ganha rótulo `confirmada: "Confirmada"`.

### D6.6 Testes
- Unit domínio: transições novas (`agendada→confirmada→realizada`, `confirmada→cancelada`), rejeição de transições inválidas, `ValidCancelBy`, `SessionCategory`.
- Unit serviço: `Confirm` grava `confirmed_at`; `Cancel` exige autor e grava metadados; audit sem conteúdo clínico.
- Migration 0004: base com e sem dados; dados preservados após rebuild; `confirmada` aceito pelo novo CHECK.
- E2E: `POST /sessions/{id}/confirm`; `POST /sessions/{id}/cancel` com/sem autor (negado sem autor).

---

## D7. Métricas do paciente e relatório por local (R6.4–R6.5, R7)

### D7.1 Motor de agregação compartilhado
- Novo serviço de relatório (`internal/service/report.go` ou similar) com um agregador que, dado um conjunto de sessões num intervalo, devolve a contagem por `SessionCategory` (D6.4) e, quando pedido, o total faturado das realizadas (soma dos débitos vinculados).
- O agregador agrupa pela **data de início** (`starts_at`) no fuso `America/Sao_Paulo`.
- Reaproveitado pelas duas dimensões (paciente e local), mudando só o filtro de origem das sessões.

### D7.2 Métricas do paciente (R6.4–R6.5)
- Repositório: reusar `sessions.ListByPatient` (+ filtro de intervalo quando houver recorte); débitos via `debts.ListByPatient`.
- Serviço: `PatientSessionMetrics(ctx, patientID, from, to)` → `{counts por categoria, total_sessoes, taxa_comparecimento, taxa_cancelamento, faturado_realizadas}`.
  - Taxa de comparecimento = realizadas / (realizadas + faltas + canceladas_paciente); zero-safe (0 quando denominador 0).
  - Taxa de cancelamento = (canceladas_paciente + canceladas_terapeuta) / total; zero-safe.
- Endpoint: `GET /v1/admin/patients/{id}/session-metrics?from=&to=` (intervalo opcional; default = histórico completo). Envelope padrão; `metricsView` serializa contagens + taxas (as taxas como fração 0..1; a UI formata em %).
- UI: bloco de KPIs no detalhe do paciente (reusa classe `.kpis`/`.kpi` já existente), visível na timeline/dados; rótulos PT-BR.

### D7.3 Relatório por local (R7)
- Repositório: novo `sessions.ListByLocationRange(ctx, locationID, from, to)` (filtra por `location_id` + `starts_at` no intervalo). Débitos das realizadas: join por `session_id` ou lookup via `debts` por sessão.
- Serviço: `LocationSessionReport(ctx, locationID, from, to)` → `{location, período, counts por categoria, total_sessoes, faturado_realizadas}`.
- Endpoint: `GET /v1/admin/locations/{id}/session-report?from=&to=` (default = mês atual). Swagger atualizado.
- UI: nova tela/seção no admin (ex.: dentro de "Locais" ou junto de "Custos e ROI"), seletor de local + navegação de período, tabela de categorias + total + faturado, rótulos PT-BR. Reaproveita o padrão visual de "Custos e ROI".

### D7.4 Período e defaults
- Default do relatório de local: **mês atual** (primeiro ao último dia, fuso local). Métricas do paciente: **histórico completo** por padrão, com recorte opcional.
- Agrupamento sempre por `starts_at` (não `created_at`). Registrado em `docs/decisions.md`.

### D7.5 Testes
- Unit do agregador: contagem por categoria (incluindo cancelado sem autor/legado), taxas zero-safe, faturado só de realizadas.
- E2E: popular sessões variadas (realizada/falta/cancelada paciente/cancelada terapeuta/agendada/confirmada/solicitada) → `GET /patients/{id}/session-metrics` e `GET /locations/{id}/session-report` conferem contagens, taxas e faturado.
- Smoke de parse dos templates novos.

---

## D8. Feed do dia do terapeuta (R8)

### D8.1 Posição e navegação
- Nova visão **"Hoje"** como tela inicial do admin, convivendo com a agenda (grade semana/mês), que permanece acessível pelo menu. A home atual (`admin_home.html`) e a seção `#agenda` seguem existindo; o feed vira a entrada padrão do app (`#hoje`).
- Seletor de janela temporal: "Hoje" (default) · "Próximos 7 dias" · "Recentes" — muda o intervalo consultado.

### D8.2 Backend: endpoint agregado
- Novo endpoint `GET /v1/admin/day-feed?window=hoje|proximos7|recentes` (ou `from`/`to` explícitos).
- Novo serviço `DayFeedService` (ou método no serviço de relatório) que compõe o feed transversal:
  - **Eventos** normalizados `{tipo, data, patient_id, patient_name, titulo, resumo, status?, valor?, ref}` — reaproveita a normalização de eventos da timeline (D4), mas sobre **todos os pacientes** no intervalo.
  - Fontes: sessões (realizadas/faltas/canceladas/agendadas/confirmadas por `starts_at` no intervalo), pagamentos (`paid_at`), débitos (`created_at` e `due_date` próximos), arquivos (`created_at`), aprovações (audit/estado do paciente), pedidos de agendamento pendentes.
  - **Resumo agregado** do período: reaproveita o motor de agregação de D7.1 (`SessionCategory` + faturado) para montar o bloco "Resumo do dia" (contagens + recebido + a receber).
- Ordenação: futuro próximo no topo decrescente até o passado recente, OU seções (futuro ↑ / passado ↓) — ver D8.3. Leitura de conteúdo clínico (se houver) gera `AuditActionRead`; metadata de audit sem conteúdo clínico.
- `dayFeedView` serializa `{resumo, eventos[]}`.

### D8.3 UI (Alpine)
- Novo componente `dayFeedPanel()` em `admin_app.html`, consumindo `GET /day-feed`.
- Agrupamento por faixa temporal legível: "Agora", "Mais tarde hoje", "Amanhã", "Já passou" (derivado da data do evento vs `clock.Now()` no fuso local).
- Cards didáticos: paciente + data relativa (`Fmt.relative`) + absoluta no title + tipo/status/valor; clique abre o paciente/sessão (reusa `$store.nav` + `openProntuarioSession`).
- **Resumo colapsado:** o bloco "Resumo do dia" (agregado) nasce **recolhido** (`x-data="{ open: false }"`, `aria-expanded`), expande sob demanda; os eventos individuais ficam visíveis. Qualquer outro bloco agregado segue a mesma regra.
- Acessibilidade: controles de expandir/colapsar operáveis por teclado, `aria-expanded`, contraste AA.

### D8.4 Reaproveitamento
- Normalização de eventos: compartilhada com a timeline por-paciente (D4) — mesma forma de item, filtro de origem diferente (um paciente × todos).
- Agregação: compartilhada com métricas (D7.2) e relatório por local (D7.3) — mesmo `SessionCategory` e cálculo de faturado.

### D8.5 Testes
- Unit do agregador do feed: eventos de todos os tipos no intervalo; separação passado/futuro; resumo correto; sem conteúdo clínico em metadata.
- E2E: popular sessões/pagamentos/arquivos em datas variadas → `GET /day-feed?window=hoje` retorna eventos e resumo esperados; `proximos7`/`recentes` mudam o conjunto.
- Smoke de parse do template; verificação manual do estado colapsado do resumo.

---

## D9. Impacto, migrations e documentação (consolidado)

- **Migrations novas:**
  - `0003_plan_due_day.sql` (D1.1) — aditiva.
  - `0004_session_status_meta.sql` (D6.2) — aditiva + **rebuild da tabela `session`** para aceitar `confirmada` no CHECK (SQLite não altera CHECK por ALTER). Preserva dados, FKs e índices. Idempotente no boot (framework aplica uma vez).
- **Contrato de API:**
  - Plano: campo `due_day`.
  - Sessão: campos `confirmed_at`, `canceled_by`, `cancel_reason`, `canceled_at` no `sessionView`; novo `POST /sessions/{id}/confirm`; corpo de `POST /sessions/{id}/cancel` com `{canceled_by, cancel_reason}`.
  - Novos endpoints: `GET /patients/{id}/history`, `GET /patients/{id}/session-metrics`, `GET /locations/{id}/session-report`, `GET /day-feed`.
  - Nova ação de audit: `session.reschedule`, `session.confirm`, `session.cancel`.
  - Atualizar Swagger (`internal/api/swagger`).
- **Docs (mesmo ciclo):** `docs/requirements.md` (débito por sessão/ciclo com vencimento; status da sessão e métricas; relatório por local; timeline; título), `docs/architecture.md` (endpoints novos, dia de vencimento, metadados de sessão, motor de agregação, rebuild da `session`), `README.md` (uso: tela base do paciente, métricas, relatório por local), `docs/decisions.md` (opt-out do `Bill`, dia de vencimento, Opção B para status, rebuild da `session` na 0004, agrupamento por `starts_at`, composição do history no servidor, CSV fora de escopo), `docs/release-notes.md` (fechamento da onda).
- **Semântica preservada:** idempotência de débito (duas chaves distintas), auditoria de leitura clínica, fuso `America/Sao_Paulo`, dinheiro em centavos.
