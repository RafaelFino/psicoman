# patient-timeline — Requisitos

Terceiro ciclo de evoluções do Psicoman. Nasce de uma rodada de feedback de uso: o terapeuta quer que a **tela do paciente seja a história completa dele**, no formato de um feed cronológico (estilo linha do tempo de rede social), e que o faturamento por sessão seja automático e consistente.

> Fonte de verdade durável: [`docs/requirements.md`](../../../docs/requirements.md). Esta spec **acrescenta** e **detalha** requisitos; onde houver conflito, o comportamento aqui descrito prevalece para o escopo `patient-timeline`.

Convenções (herdadas de `mvp-audit1`):
- Requisitos em EARS ("O sistema DEVE …", "QUANDO … o sistema DEVE …").
- Toda mensagem ao usuário em **PT-BR**, tom acolhedor, zero jargão (público leigo — `docs/requirements.md §4.5`).
- Valores monetários em centavos (BRL); datas no fuso `America/Sao_Paulo`.
- Backend imposto no servidor (defense in depth); a UI apenas reflete.

Resumo das evoluções desta onda:
1. **R1** — Débito automático a cada sessão realizada (e por ciclo, a partir do dia de vencimento do plano).
2. **R2** — Calendário mensal das sessões do paciente (passado + futuro) na tela do paciente.
3. **R3** — Feed de prontuário (anotações do terapeuta) ordenado do mais recente ao mais antigo, com a Anamnese como primeiro/último marco fundacional.
4. **R4** — Timeline unificada da história do paciente (sessões, registros, débitos, pagamentos, arquivos, remarcações, aprovação) como tela base do paciente.
5. **R5** — Título da página sempre com a marca "Psicoman".
6. **R6** — Controle completo de status da sessão (confirmação, autor/motivo de cancelamento) e métricas de sessão no prontuário do paciente.
7. **R7** — Relatório de sessões por local (mensal), por categoria + volume financeiro.
8. **R8** — Feed do dia do terapeuta (transversal a todos os pacientes), com cards didáticos e resumos colapsáveis.

---

## R1. Débito automático por sessão realizada

**Contexto:** hoje o débito por sessão só é gerado em `BillingService.OnSessionFinished`, **se e somente se** o flag `Bill` da sessão estiver marcado na finalização *e* houver plano por-sessão vigente (`internal/service/billing.go`). O terapeuta quer que, ao marcar uma sessão como **realizada**, o débito correspondente seja criado automaticamente, sem depender de lembrar de marcar o flag.

### R1.1 Geração na finalização
- QUANDO uma sessão transita para `realizada`, o sistema DEVE gerar o débito correspondente de forma automática, sem exigir ação manual adicional do terapeuta.
- A geração DEVE ser **idempotente** por sessão (reprocessar a mesma sessão não cria débito duplicado), reusando `SessionIdempotencyKey` e o índice único `ux_debt_idempotency` já existentes.
- O débito gerado DEVE vincular `patient_id`, `session_id` e `plan_id`, com `amount` derivado do plano vigente na data de início da sessão e `due_date` conforme a política atual (hoje: +7 dias).

### R1.2 Regras por tipo de plano
- QUANDO o paciente tem **plano por sessão** (`pagamento_por_consulta` / `pagamento_por_mes` cobrado por sessão), o sistema DEVE gerar o débito **na finalização da sessão**, idempotente por `session:<id>`.
- QUANDO o paciente tem **plano fechado** (mensal/trimestral), o sistema NÃO DEVE gerar débito por sessão. A cobrança DEVE ocorrer **por ciclo**, disparada **a partir do dia de vencimento** do plano naquele período (ver R1.5), idempotente por `cycle:<plan_id>:<período>`, para nunca cobrar em duplicidade.
- QUANDO o paciente é **atendimento social**, o sistema NÃO DEVE gerar débito.
- QUANDO não há plano vigente na data da sessão (planos por sessão), o sistema NÃO DEVE gerar débito (não há base de valor) e DEVE registrar essa condição de forma observável (log/audit), para o terapeuta perceber que faltou cadastrar o plano.

### R1.3 Relação com o flag `Bill`
- O comportamento padrão DEVE deixar de depender do terapeuta marcar o flag `Bill` manualmente: toda sessão `realizada` de paciente com plano cobrável por sessão gera débito.
- O flag `Bill` DEVE passar a funcionar como **opt-out explícito** ("esta sessão não gera cobrança"): quando desmarcado deliberadamente, suprime a geração; o padrão de uma sessão realizada é gerar o débito.
- O flag `ConsiderCost` (rateio de custo) permanece com a semântica atual e fora do escopo desta onda.

### R1.4 Visibilidade
- QUANDO um débito é gerado por uma sessão realizada, ele DEVE aparecer na aba/visão de débitos do paciente e como evento na timeline (R4), vinculado à sessão de origem.
- QUANDO um débito de plano fechado é gerado por ciclo, ele DEVE aparecer na visão de débitos e na timeline, identificado pelo período de faturamento.

### R1.5 Dia de vencimento dos planos fechados
- O **plano** DEVE passar a ter um **dia de vencimento** (`due_day`, 1–28 para evitar meses curtos), persistido por migration aditiva, com valor padrão coerente para os planos pré-existentes.
- O job de fechamento de ciclo (`CloseCycles`, cron diário) DEVE gerar o débito do período vigente **somente quando a data de referência alcançar o dia de vencimento** daquele período (`ref.Day() >= due_day`).
- A idempotência por `cycle:<plan_id>:<YYYY-MM>` (mensal) / `cycle:<plan_id>:<YYYY-Q>` (trimestral) DEVE garantir **um único débito por período por plano**, independentemente de quantas vezes o cron rodar após o vencimento.
- A `due_date` do débito gerado por ciclo DEVE refletir o **dia de vencimento real** do período (não mais `now + 7 dias` arbitrário).
- O gatilho DEVE permanecer o **cron diário** existente; não há geração preguiçosa (lazy) nesta onda.

---

## R2. Calendário mensal do paciente

**Contexto:** hoje a agenda é global (`#agenda`, visões semana/mês). Na tela do paciente não há uma visão de calendário com as sessões **daquele** paciente. O terapeuta quer, ao abrir um paciente, enxergar num calendário mensal todas as sessões dele — passadas e futuras.

### R2.1 Visão de calendário na tela do paciente
- A tela do paciente DEVE oferecer uma visão de **calendário mensal** com todas as sessões daquele paciente.
- O calendário DEVE incluir sessões **passadas** (realizadas, faltas, canceladas) e **futuras** (agendadas, solicitadas).
- Cada dia com sessão(ões) DEVE indicar visualmente a presença e o **status** da sessão, com cores coerentes com a semântica já usada (realizada=ok, agendada/solicitada=info, falta=atenção, cancelada=perigo).
- O terapeuta DEVE poder **navegar entre meses** (anterior/próximo) e voltar ao mês atual.
- QUANDO há mais de uma sessão no mesmo dia, o calendário DEVE indicar a quantidade e permitir ver os detalhes do dia.
- Clicar numa sessão do calendário DEVE levar ao prontuário/registro daquela sessão (reusar o fluxo `openProntuarioSession` já existente).

### R2.2 Dados
- O backend DEVE fornecer as sessões do paciente num intervalo (reusar `GET /v1/admin/sessions?patient_id=…` com filtro `from`/`to`, já existente), suficiente para montar a grade do mês.
- A grade mensal DEVE respeitar o fuso `America/Sao_Paulo` ao agrupar sessões por dia.

---

## R3. Feed de prontuário (anotações do terapeuta)

**Contexto:** as anotações do terapeuta são o coração clínico (`Note` + `Anamnesis` em `internal/domain/prontuario.go`). O terapeuta quer lê-las como um feed: do **mais recente ao mais antigo**, com a **Anamnese** como marco fundacional (o "primeiro registro" da história clínica).

### R3.1 Ordenação
- O feed de prontuário DEVE ordenar os registros do **mais recente para o mais antigo** (desc por data).
- A **Anamnese** DEVE ser apresentada como o registro fundacional da história clínica (o primeiro contato clínico), destacada visualmente dos demais registros.
- A ordenação cronológica reversa DEVE valer tanto para notas de sessão quanto para notas livres.

### R3.2 Formato de feed
- Cada item do feed DEVE exibir: data/hora (relativa e absoluta), tipo (anamnese, registro de sessão, nota livre), e o conteúdo renderizado em Markdown (reusar `Md.toHtml`).
- Registros de sessão DEVEM exibir o contexto da sessão (data, modalidade, status) junto do conteúdo.
- O feed DEVE permitir **adicionar uma nota livre** rapidamente a partir do topo (campo de composição estilo "o que aconteceu?").
- O feed DEVE preservar as operações atuais de prontuário (editar/salvar anamnese, adicionar/remover nota), sem perda de funcionalidade.

### R3.3 Acolhimento e leigo
- Rótulos e microcópia DEVEM ser em PT-BR, claros e sem jargão. Datas relativas ("há 2 dias", "hoje") DEVEM acompanhar a data absoluta no title/tooltip.

---

## R4. Timeline unificada da história do paciente (tela base)

**Contexto:** o terapeuta quer que a tela base do paciente seja a **história completa**: desde o primeiro contato, cada sessão, cada relatório, cada débito, cada pagamento, cada arquivo anexado, cada remarcação. Um feed único que conte a vida do paciente no consultório.

### R4.1 Tela base = timeline
- A tela do paciente DEVE ter, como **visão inicial/base**, uma **timeline unificada** com todos os eventos da história do paciente, ordenada do mais recente ao mais antigo.
- A timeline DEVE agregar, no mínimo, os seguintes tipos de evento, cada um com data, ícone/cor e um resumo legível:
  - **Cadastro / primeiro contato** (criação do paciente; aprovação/rejeição quando houver).
  - **Sessão** (solicitada, agendada, realizada, falta, cancelada) com data e modalidade.
  - **Remarcação** (alteração de data/horário de uma sessão agendada).
  - **Registro clínico** (nota de sessão) e **nota livre**.
  - **Anamnese** (criação/atualização).
  - **Débito** gerado (com valor e origem — sessão ou ciclo).
  - **Pagamento** registrado (com valor e método).
  - **Arquivo anexado** (GED), com nome/tipo e link de download.
- Cada evento DEVE ser clicável quando houver detalhe/ação associada (abrir a sessão no prontuário, abrir o débito, baixar o arquivo).

### R4.2 Composição dos dados
- A timeline DEVE ser composta a partir das fontes já existentes (sessões, notas, anamnese, débitos, pagamentos, arquivos, estado de aprovação), evitando duplicar dados ou criar uma nova tabela de "eventos" se puder ser derivada.
- O backend DEVE oferecer os dados necessários por endpoints já existentes do paciente; onde faltar (ex.: remarcação não é registrada hoje de forma rastreável), o design DEVE propor o mínimo necessário (ver R4.3).

### R4.3 Rastreabilidade de remarcação
- QUANDO uma sessão agendada tem sua data/horário alterada, o sistema DEVE registrar essa alteração de forma rastreável, suficiente para exibir uma entrada de "remarcação" na timeline (de/para).
- O registro de remarcação DEVE constar no **audit log** (operação sensível) e/ou numa forma consultável pela timeline do paciente.

### R4.4 Navegação e abas
- A timeline DEVE ser a aba/visão padrão ao abrir o paciente, convivendo com as abas atuais (Dados, Anamnese, Prontuário, Plano, Débitos, Custos, Arquivos) e com o calendário mensal (R2).
- O terapeuta DEVE poder **filtrar** a timeline por tipo de evento (ex.: só financeiro, só clínico), sem perder a visão completa como padrão.

### R4.5 Privacidade e auditoria
- A timeline DEVE respeitar as mesmas regras de auditoria de leitura clínica já aplicadas no prontuário (toda leitura de conteúdo clínico gera `AuditActionRead`).
- A timeline NÃO DEVE expor conteúdo clínico em metadados de auditoria (coerente com `docs/requirements.md §4.1`).

---

## R5. Título da página com a marca "Psicoman"

**Contexto:** o `<title>` da aba do navegador no app admin é "Painel" (`internal/web/web.go`, `adminApp`). A marca "Psicoman" só aparece na topbar. O terapeuta quer que o título da página sempre carregue o nome da aplicação.

### R5.1 Título com marca
- O `<title>` de todas as páginas DEVE incluir o nome "Psicoman".
- O título DEVE seguir o padrão `Psicoman · <contexto>` (ex.: `Psicoman · Painel`, `Psicoman · Portal do Paciente`), com a marca sempre presente e o contexto quando houver.
- A mudança DEVE valer para admin e portal, em todas as páginas renderizadas (`index`, `adminApp`, `portalApp`).
- A topbar DEVE permanecer coerente (marca "Psicoman" + subtítulo de contexto), sem duplicar informação de forma confusa.

---

## R6. Controle de status da sessão e métricas no prontuário

**Contexto:** a máquina de estados atual (`internal/domain/session.go`) tem `solicitada → agendada → {realizada, cancelada, falta}`. Ela não distingue **quem cancelou** (paciente × terapeuta), não registra **quando** o cancelamento ocorreu nem o **motivo**, e não há um passo explícito de **confirmação** do terapeuta. O terapeuta quer visão completa do ciclo de vida da sessão e métricas por paciente.

Escolha de modelagem (confirmada): **Opção B — status enxuto + metadados ortogonais** (não desdobrar o status por autor; registrar autor/motivo/datas em campos). Registrada em `docs/decisions.md`.

### R6.1 Estado de confirmação
- A sessão DEVE suportar um estado explícito `confirmada`, entre `agendada` e os desfechos.
- Transições válidas DEVEM passar a ser:
  - `solicitada → agendada | cancelada`
  - `agendada → confirmada | realizada | cancelada | falta`
  - `confirmada → realizada | cancelada | falta`
  - `realizada | cancelada | falta` → terminais.
- QUANDO o terapeuta confirma uma sessão, o sistema DEVE registrar `confirmed_at` (fuso `America/Sao_Paulo`).
- A confirmação DEVE ser opcional no fluxo: uma sessão `agendada` ainda pode ir direto a `realizada`/`falta`/`cancelada` (nem toda operação exige confirmação prévia).

### R6.2 Metadados de cancelamento
- QUANDO uma sessão é cancelada, o sistema DEVE registrar: `canceled_by` (`paciente` | `terapeuta`), `cancel_reason` (texto livre opcional, PT-BR) e `canceled_at`.
- O status permanece `cancelada` (um único estado); o autor e o motivo são metadados.
- `canceled_by` DEVE ser obrigatório ao cancelar; `cancel_reason` DEVE ser opcional.
- O cancelamento DEVE registrar entrada no **audit log** (operação sensível), sem conteúdo clínico nos metadados.

### R6.3 Persistência e migration
- Os campos `confirmed_at`, `canceled_by`, `cancel_reason`, `canceled_at` DEVEM ser persistidos por migration aditiva e idempotente no boot, nulos para registros pré-existentes.
- O `CHECK` de `status` na tabela `session` DEVE passar a aceitar `confirmada`.

### R6.4 Métricas no prontuário do paciente
- Ao abrir o paciente, o sistema DEVE exibir um bloco de **métricas de sessão** com a quebra por categoria derivada de `status + canceled_by`:
  - **Realizadas** (cobráveis), **Faltas** (no-show), **Canceladas pelo paciente**, **Canceladas pelo terapeuta**, **Agendadas/Confirmadas** (futuras), **Solicitadas** (pendentes de agendar).
- O bloco DEVE exibir taxas úteis:
  - **Taxa de comparecimento** = realizadas / (realizadas + faltas + canceladas pelo paciente).
  - **Taxa de cancelamento** = (canceladas paciente + canceladas terapeuta) / total no período.
- As métricas DEVEM respeitar um intervalo (default: histórico completo do paciente, com opção de recorte por período), agrupando pela **data de início** da sessão (`starts_at`) no fuso `America/Sao_Paulo`.
- As categorias e taxas DEVEM ter rótulos PT-BR claros, sem jargão.

### R6.5 Dados
- O backend DEVE fornecer as métricas agregadas do paciente via endpoint próprio (ver design D8), não computadas apenas no cliente, para servir também a testes de contrato.

---

## R7. Relatório de sessões por local

**Contexto:** é comum o local ser uma clínica parceira que, mensalmente, quer saber quantos atendimentos ocorreram, quantos foram cancelados, quantos seguem agendados. O schema já suporta: `session.location_id` existe.

### R7.1 Relatório agregado por local
- O admin DEVE ter um relatório que, para um **local** e um **intervalo** (default: mês atual), agregue as sessões por categoria derivada (as mesmas de R6.4): realizadas, faltas, canceladas pelo paciente, canceladas pelo terapeuta, agendadas/confirmadas, solicitadas.
- O agrupamento temporal DEVE usar a **data de início** da sessão (`starts_at`) no fuso `America/Sao_Paulo`.
- O relatório DEVE permitir navegar por período (mês anterior/próximo, ou intervalo personalizado).

### R7.2 Volume financeiro
- O relatório DEVE incluir o **total faturado** das sessões realizadas no período para aquele local (soma dos débitos vinculados às sessões realizadas), em centavos (BRL), exibido em reais.
- A exibição do valor financeiro DEVE ser clara quanto ao que representa (faturamento das realizadas no período, não recebimento).

### R7.3 Experiência e exportação
- O relatório DEVE ser legível para repasse à clínica (contagens por categoria + total + período), com rótulos PT-BR.
- O relatório DEVE reaproveitar o padrão de tela já existente de "Custos e ROI" no admin (consistência visual).
- A exportação (CSV/impressão) é desejável, mas **fora do escopo mínimo** desta onda; pode ser registrada como melhoria futura em `docs/decisions.md`.

### R7.4 Dados
- O backend DEVE fornecer o relatório agregado por local via endpoint próprio (ver design D8), compartilhando o motor de agregação com as métricas do paciente (R6.5).

---

## R8. Feed do dia do terapeuta

**Contexto:** hoje o admin abre na agenda (`#agenda`, grade semana/mês) — uma lista de horários, não um resumo didático. O terapeuta quer um **feed do seu dia**, transversal a todos os pacientes, que conte "o que aconteceu" e "o que vem", de forma legível, com cards ricos em detalhe. Distinto do feed por-paciente (R3/R4), que é a história de **um** paciente.

### R8.1 Escopo e posição
- O admin DEVE ter um **feed do dia** centrado em "hoje", transversal a **todos os pacientes**.
- O feed DEVE ser a **visão inicial do admin** (tela de entrada do app), convivendo com a agenda (grade) como visão alternativa — sem remover a agenda atual.
- O feed DEVE oferecer uma **janela temporal ajustável**: "Hoje" (default), "Próximos 7 dias" e "Recentes" (passado recente).

### R8.2 Conteúdo (o que aconteceu + o que vem)
- O feed DEVE agregar, no período selecionado e no fuso `America/Sao_Paulo`:
  - **Passado/aconteceu:** sessões realizadas, faltas, cancelamentos (com autor), pagamentos recebidos, débitos gerados, arquivos anexados, pacientes aprovados/rejeitados.
  - **Futuro/vai acontecer:** próximas sessões agendadas/confirmadas, pedidos de agendamento pendentes, vencimentos de débito próximos.
- Cada evento DEVE identificar o **paciente** associado e permitir abrir o paciente/sessão correspondente (navegação para a timeline/prontuário).

### R8.3 Cards didáticos
- Cada card DEVE trazer detalhe suficiente para entender o evento **sem precisar clicar**: quem (paciente), quando (data/hora relativa + absoluta), o quê (tipo), status, e valor quando financeiro.
- A linguagem DEVE ser PT-BR, acolhedora e sem jargão.
- O feed DEVE agrupar os cards por faixa temporal legível (ex.: "Agora", "Mais tarde hoje", "Amanhã", "Já passou"), com separadores visuais claros.

### R8.4 Cards de resumo colapsados
- Blocos de **resumo/agregado** (ex.: "Resumo do dia: X realizadas, Y faltas, Z recebido") DEVEM aparecer **colapsados por padrão**, expandindo sob demanda, para não poluir a interface.
- Os **eventos individuais** (sessões, pagamentos, etc.) DEVEM aparecer **expandidos** (visíveis diretamente); apenas os blocos de resumo/agregado nascem recolhidos.
- O estado de expandido/colapsado DEVE ser controlável pelo terapeuta por bloco, com controles acessíveis (teclado + `aria-expanded`).

### R8.5 Dados
- O backend DEVE fornecer o feed do dia agregado via endpoint próprio (ver design D8), não computado apenas no cliente, reaproveitando o motor de agregação (R6.5/R7.4) para os blocos de resumo.
- A leitura de conteúdo clínico (se exibido em algum card) DEVE respeitar a auditoria de leitura (`AuditActionRead`); metadados de auditoria NÃO DEVEM conter conteúdo clínico.
