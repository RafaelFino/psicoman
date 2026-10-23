# Psicoman — Registro de Decisões (ADR leve)

Lastro das decisões relevantes de produto e arquitetura. Formato: data, contexto, decisão, consequências. Política em [`.kiro/steering/psicoman-processo-entrega.md`](../.kiro/steering/psicoman-processo-entrega.md).

Ordem: mais recente no topo.

---

## 2026-10-21 — Onda "pocketbase-core": PocketBase como framework, binário único, domínio rico e cripto por campo

**Contexto.** A V1 do Psicoman **ainda não foi publicada** — não há lastro, não há dados reais em produção e a base técnica pode ser reescrita à vontade. A plataforma é **single-tenant** (uma VM por terapeuta) e roda **sempre atrás do [`Pangolin`](https://docs.fossorial.io/)**, nunca exposta diretamente à internet. A demanda de concorrência é mínima: um único terapeuta operando e, nos dias mais agitados, dois ou três cadastros de paciente. Ao longo da evolução anterior ficou claro que faltavam **invariantes transacionais** (unicidade de email/CPF, FK, transação, idempotência de efeito financeiro) que as ideias de arquitetura exploradas até aqui não entregavam de forma barata. Esta entrada fixa a arquitetura-alvo da onda `pocketbase-core` e a **nomenclatura canônica** que as specs e steerings desta onda consomem.

**Alternativas consideradas e descartadas.** Avaliamos (não como passado a corrigir, mas como caminhos possíveis) três arquiteturas antes de escolher a atual:

- **Alt.1 — SQLite local primário puro (da POC original).**
  - *Prós:* simples, zero dependência externa, controle total do schema e das transações; invariantes transacionais nativos.
  - *Contras:* não entrega HTTP server, file storage/server, auth/OAuth2, admin UI nem migrations prontos — tudo isso teria de ser reimplementado à mão, inflando o código de infraestrutura em vez de código de negócio.
  - *Motivo do descarte:* reescreveríamos à mão várias peças que um framework já resolve, sem ganho de negócio.

- **Alt.2 — Google Drive como fonte da verdade** (ver `/tmp/evolucao-ref.md`).
  - *Prós:* acervo legível e portável, backup implícito, nenhum banco a operar.
  - *Contras:* **sem constraints/FK/UNIQUE/transação**, consistência apenas eventual, sujeito a rate-limit e latência de rede no caminho de request. Inviável para invariantes financeiros (idempotência de débito) e de unicidade.
  - *Motivo do descarte:* perde exatamente as garantias transacionais que motivam a onda; o Drive no caminho de escrita torna cada operação de negócio frágil.

- **Alt.3 — Corte híbrido "acervo no Drive + controle no SQLite".**
  - *Prós:* tira o conteúdo pesado (markdown clínico) do banco e mantém o controle relacional (sessões, débitos) com constraints.
  - *Contras:* mantém o Drive no **caminho de escrita/verdade** do acervo e exige **sync bidirecional**; passam a existir dois lados de verdade a reconciliar, com janelas de inconsistência.
  - *Motivo do descarte:* a complexidade de sync bidirecional e o duplo lado de verdade não compensam frente a um único banco transacional.

**Motivo geral do descarte:** nenhuma dessas alternativas recupera, de forma barata, **invariantes transacionais + HTTP + file-server + auth + admin num binário único** atrás do `Pangolin`. É isso que a decisão abaixo entrega.

**Decisão.** Adotamos o **PocketBase como framework** (biblioteca Go), e esta passa a ser a **arquitetura padrão da V1** — substitui integralmente as explorações anteriores (sem lastro, porque a plataforma não foi publicada). Nove pontos:

1. **PocketBase como framework → binário único.** Importamos o PocketBase como lib Go em `cmd/psicoman/main.go` (`pocketbase.New()` + hook `app.OnServe()` para registrar as rotas próprias). Aproveitamos do PB o SQLite relacional com constraints/relations, o HTTP server, o file storage/server, o logging, o auth/OAuth2, as migrations e a admin UI. Isso recupera os invariantes transacionais (UNIQUE, FK, ON CONFLICT, transação) que a ideia de Drive-como-banco não dava.
2. **Binário único** substitui os antigos `cmd/admin` + `cmd/portal`. A segregação admin/portal deixa de ser **física** (dois processos) e passa a ser **lógica** (grupos de rota + regras de auth), já que tudo roda atrás do `Pangolin` e nunca é exposto direto à internet. Um binário simplifica o roll out.
3. **Auth simplificada.** O `Pangolin` é a barreira forte de borda; o app confia no gateway e mantém só o mínimo de defense-in-depth. Expomos de forma controlada apenas cadastro/login do paciente. A identidade do admin chega via header do `Pangolin`. O **gate de aprovação do paciente** (`pendente`/`aprovado`/`rejeitado`, com enforcement no servidor via hook) permanece.
4. **Domínio rico** substitui o "domínio magro". Agregados por bounded context carregam comportamento, serialização/desserialização, formatação de log e ações no próprio domínio (OO bem feita); os serviços de negócio ficam enxutos, apenas orquestrando. Bounded contexts: `patient`, `clinical`, `billing`, `scheduling`, `costs`, `therapist`.
5. **Ports & Adapters — `platform/pb`.** O mapeamento domínio↔Record do PocketBase fica isolado no pacote `internal/platform/pb`. Os agregados de domínio são a autoridade e **não** dependem do PocketBase; trocar o PB por outra base significa reescrever só o adapter.
6. **Cripto por campo (`SealedText`).** Campos clínicos sensíveis (corpo da sessão, anamnese, notas, resumo de IA) usam o tipo wrapper `SealedText`, cuja serialização para persistência produz o envelope `versao_do_esquema.algoritmo.id_da_chave.nonce.ciphertext` (base64) e cujo `MarshalJSON` emite sempre o envelope **cifrado**, nunca o claro; o claro só sai via `Reveal(ctx)`. Algoritmo **AES-256-GCM**, pacote `internal/platform/crypto`, chave via config atrás de um `KeyProvider` (o `id_da_chave` no envelope habilita rotação; `versao_do_esquema`/`algoritmo` habilitam evolução sem quebrar dados antigos). A cifra é **por campo**: nome, email, telefone, datas, status e valores ficam em claro e indexáveis; a busca clínica se dá pelas **tags** da IA (em claro), não pelo corpo (cifrado). O PocketBase expõe `security.Encrypt`/`security.Decrypt` (AES-256-GCM, chave de 32 chars), mas o PB **não** cifra arquivo/DB sozinho — a cifra é do **nosso** `SealedText`.
7. **Contrato de API revisado do zero.** Rotas próprias sob o prefixo `/v1/*` (namespaces lógicos `admin`/`portal`) registradas sobre o router do PocketBase; a REST automática do PB **não** é o contrato público. Envelope de resposta `{ message, elapsed_ms, data, error }` (`message` em PT-BR), rotas versionadas e Swagger.
8. **Integrações.** O Google Calendar permanece **síncrono** (agenda/Meet, checagem de conflito via freebusy antes de confirmar a sessão) — é o único Google no caminho de request. O Google Drive **deixa de ser síncrono** e vira **fallback/DR**: um hook no ciclo de **backup** do PocketBase dispara o sync do snapshot para o Drive (em `internal/drivesync`, podendo ser segundo processo/hook); o Drive nunca mais é fonte da verdade nem caminho de request. A IA fica em `internal/integration/gemini` (Gemini pago com proteção de dados): resumo clínico + tagueamento com índice invertido de tags por paciente/ano, disparo explícito pelo terapeuta, resultado revisável antes de consolidar, **opt-in por instância** (default off), auditoria `ai.process` sem conteúdo clínico e minimização. Base legal: os dados são da própria terapeuta (controladora), que já faz esse trabalho manualmente.
9. **Persistência.** Coleções PocketBase com constraints/relations recuperam unicidade (email/CPF), idempotência de débito (chave `UNIQUE`), FK e transação. Os padrões de domínio permanecem não-negociáveis: ULID em toda entidade, dinheiro `int64` em centavos BRL, tempo `America/Sao_Paulo` em ISO-8601, idempotência de efeito financeiro por chave única.

**Estrutura de pastas alvo:** `cmd/psicoman/main.go` (único); `internal/domain/{patient,clinical,billing,scheduling,costs,therapist}` + o tipo `SealedText`; `internal/app` (serviços enxutos); `internal/platform/{crypto,pb}`; `internal/integration/{google,gemini}`; `internal/drivesync`; `internal/api` (rotas `/v1/*` próprias, envelope PT-BR, sobre o router do PB).

**Consequências.**
- **Steerings reescritos nesta onda:** [`psicoman-golang.md`](../.kiro/steering/psicoman-golang.md) (domínio rico + binário único + estrutura de pastas nova), com ajustes alinhados em [`psicoman-seguranca-lgpd.md`](../.kiro/steering/psicoman-seguranca-lgpd.md) (binário único com segregação lógica, gate de aprovação via hook, `SealedText` por campo, Drive como fallback/DR) e [`psicoman-google-api.md`](../.kiro/steering/psicoman-google-api.md) (Calendar síncrono, Drive fallback via hook de backup).
- **Nasce a spec** [`.kiro/specs/pocketbase-core/`](../.kiro/specs/pocketbase-core/requirements.md) (requirements/design/tasks) descrevendo a reescrita por fases.
- **Binário único** (`cmd/psicoman`) substitui `cmd/admin` + `cmd/portal`; segregação admin/portal vira lógica.
- **Domínio rico** por bounded context; serviços enxutos em `internal/app`.
- **Cripto por campo** via `SealedText` (AES-256-GCM, envelope com `id_da_chave` para rotação, `KeyProvider`).
- **Contrato `/v1`** revisto com envelope `{ message, elapsed_ms, data, error }` e Swagger.
- **Drive vira fallback/DR** (hook de backup → `drivesync`); **Gemini opt-in** por instância.
- `docs/requirements.md` e `docs/architecture.md` **serão reescritos** para esta arquitetura pela fase de fechamento das tasks da onda (**não** por esta entrada, que cobre ADR + spec + steerings).

---

## 2026-10-13 — Observabilidade: detalhe técnico dos erros 5xx vai para o log

**Contexto.** Ao investigar o 500 do agendamento, o diagnóstico foi dificultado porque o `RespondError` escrevia o envelope de erro mas **nunca registrava o `Internal`** (detalhe técnico) — o corpo da resposta esconde o detalhe por segurança (correto) e o middleware de logging só registra o status. Resultado: 500 opacos, sem rastro da causa.

**Decisão.** O `httpx.RespondError` passa a **logar o detalhe técnico dos erros 5xx** (nível error), correlacionado por `request_id`, `code` e `path` — nunca no corpo da resposta (o `detail` ao cliente continua `null`). Erros 4xx não são logados como erro interno. Um logger de pacote é injetado no boot via `httpx.SetLogger(log)` (em `bootstrap`, vale para admin e portal); sem logger, o log é omitido sem panic. A interface `httpx.ErrorLogger` desacopla o `httpx` do pacote de logger concreto.

**Consequências.**
- `internal/api/httpx/envelope.go`: `ErrorLogger` + `SetLogger` + log do `Internal` nos 5xx em `RespondError`.
- `internal/app/app.go`: `httpx.SetLogger(log)` no `bootstrap`.
- Testes unitários (`envelope_test.go`): 5xx loga o detalhe e não vaza no corpo; 4xx não loga; sem logger não entra em panic.
- `docs/architecture.md §6`: nota de que o detalhe dos 5xx vai ao log.

---

## 2026-10-13 — Fix: agendar/backup sem Google configurado não deve dar 500

**Contexto.** Em `dev_mode` (ou numa instância ainda não autorizada no Google), criar uma sessão já como `agendada` retornava **HTTP 500**: o `buildAdmin` injetava o cliente Google **real** sempre que não havia mock, mesmo sem credenciais OAuth; ao criar o evento no Calendar, a chamada falhava no OAuth sem token. Reproduzido com o seed ("dia lotado"): as sessões `agendada` davam 500 e as `solicitada` passavam. Bug **pré-existente** (anterior à onda `patient-timeline`), exposto ao rodar o seed na stack local. O backup/restore tinha o mesmo risco: `BackupService` chamaria `drive.Upload`/`List` com `drive` nil → panic.

**Decisões.**
1. **Só ativar os clientes Google reais quando houver credenciais** (`client_id` + `client_secret`). Sem elas, `calendar`/`gmail`/`drive` ficam `nil`: o `SessionService` já trata `nil` como no-op (agenda sem evento, usa o `noopConflictChecker`), o envio de email é pulado (registro/PDF seguem) e o boot emite um `AVISO`. Alinha com `psicoman-google-api` ("sem panic", degraded) e com o propósito do `dev_mode` (rodar local sem dependências externas).
2. **`BackupService.Backup`/`Restore` toleram `drive` nil**, devolvendo erro de validação tratável ("Drive não configurado") em vez de estourar.

**Consequências.**
- `internal/app/app.go`: injeção condicional do cliente Google; `SetCalendar`/`SetConflictChecker`/sender só quando o cliente existe.
- `internal/service/backup.go`: guard de `drive == nil` em `Backup` e `Restore`.
- Testes E2E novos: `TestAgendarSemGoogle` (agendar 201 sem Google, sem Meet) e `TestBackupSemGoogle` (backup não dá 500). Harness ganhou `StartAdminNoGoogle` (admin sem mocks e sem credenciais). Corrigida também uma fragilidade de borda no `TestDayFeed` (sessão/plano ancorados ao dia local, não a `+2h`).
- Produção inalterada quando o Google está configurado.

---

## 2026-10-12 — Onda "patient-timeline" (Fase H): feed de prontuário e título com a marca

**Contexto.** Spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md) R3 e R5. O terapeuta quer ler o prontuário como feed cronológico reverso com a anamnese como marco fundacional, e que o título da aba sempre carregue a marca "Psicoman".

**Decisões.**
1. **Prontuário já era feed desc; reforço da anamnese + composição.** A aba Prontuário (herdada de `prontuario-ux`) já ordena do mais recente ao mais antigo e já tinha composição de nota livre no topo. Esta fase: a composição foi rotulada "Registrar o que aconteceu…" (R3.2) e a **Anamnese** passou a aparecer como **card fundacional destacado no topo** ("Ficha inicial") E como **marco no fim** da linha do tempo ("Início da história clínica") — evita a ambiguidade de onde ordenar a anamnese (decisão D3.1). `loadProntuario` passou a carregar a anamnese. Nenhuma regressão: editar/salvar anamnese e add/remove nota seguem iguais. Spec R3.1–R3.3 · D3.
2. **Título da aba com a marca, sem duplicar na topbar.** O `<title>` é computado no template `head` como `Psicoman · {{.Title}}` (admin e portal, todas as páginas); as strings de contexto em `web.go` ficam inalteradas e a topbar continua `Psicoman <span class="sub">{{.Title}}</span>` — assim o navegador mostra "Psicoman · Painel" sem a topbar virar "Psicoman · Psicoman · Painel". Spec R5.1 · D5.

**Consequências.**
- UI only (sem backend/contrato novo). `layout.html` head com a marca; prontuário com Ficha inicial + marco fundacional + composição rotulada.
- Testes: smoke de parse + asserts (`<title>Psicoman · ` em admin e portal; "Registrar o que aconteceu" e "Ficha inicial" no prontuário). Verificação manual do `<title>` renderizado.

---

## 2026-10-12 — Onda "patient-timeline" (Fase G): feed do dia do terapeuta

**Contexto.** Spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md) R8. O admin abria na agenda (grade de horários). O terapeuta quer um **feed do dia** transversal a todos os pacientes, didático, que conte "o que aconteceu" e "o que vem".

**Decisões.**
1. **`#hoje` é a visão inicial do app; a agenda convive.** A seção "Hoje" (`dayFeedPanel`) passa a ser a **primeira** do `admin_app.html` (logo é o que aparece ao abrir `/app/`). A agenda (grade semana/mês) continua existindo e acessível pelo menu (`#agenda`); nada foi removido. Spec R8.1 · D8.1.
2. **Composição no servidor (`DayFeedService` + `GET /day-feed?window=`).** Reaproveita a normalização de eventos da timeline por-paciente (tipos de `history.go`) sobre **todos** os pacientes num intervalo, e o motor de agregação (F1) para o bloco "Resumo do dia". Janelas: `hoje` (dia local), `proximos7` (hoje → +7 dias), `recentes` (−7 dias → fim de hoje); janela inválida cai em `hoje`. Fontes: sessões (`starts_at`), débitos (criação) + vencimentos próximos, pagamentos (`paid_at`), arquivos (`created_at`), pedidos pendentes. Spec R8.2, R8.5 · D8.2.
3. **Resumo colapsado, eventos expandidos.** O bloco agregado ("Resumo do dia": sessões/realizadas/faltas, recebido, a receber) nasce **recolhido** (`x-data open:false`, `aria-expanded`, operável por teclado); os eventos individuais ficam visíveis, agrupados por faixa temporal legível ("Agora/Já passou hoje", "Mais tarde hoje", "Amanhã", "Próximos dias", "Já passou"). Spec R8.3–R8.4 · D8.3.
4. **Sem conteúdo clínico; auditoria de leitura.** O feed carrega só metadados (paciente, tipo, status, valor), nunca conteúdo clínico. Cada carga registra `AuditActionRead` sobre `day_feed` com metadata `{window, eventos}`. Spec R8.5.

**Consequências.**
- Novo `DayFeedService` + endpoint `GET /v1/admin/day-feed`; novos repositórios de intervalo: `debts.ListByCreatedRange`/`debts.ListDueBetween`, `payments.ListByRange`, `ged.ListByCreatedRange`. Swagger atualizado.
- UI: nova seção `#hoje` (entrada do app), item de menu "Hoje"; cards didáticos com nome do paciente, data relativa/absoluta, tipo/status/valor; clique abre paciente/sessão.
- Testes: unit do `DayFeedService` (janelas, tipos de evento, resumo recebido/a-receber, ordenação, nome do paciente) + E2E (`day-feed` nas três janelas, janela inválida → hoje).

---

## 2026-10-12 — Onda "patient-timeline" (Fase F): métricas do paciente e relatório por local

**Contexto.** Spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md) R6.4–R6.5 e R7. O terapeuta quer métricas de sessão por paciente (comparecimento, cancelamento, faturado) e um relatório mensal por local para repasse a clínicas parceiras.

**Decisões.**
1. **Motor de agregação compartilhado por `SessionCategory`.** Um agregador puro (`aggregateSessions`) recebe um conjunto de sessões e um mapa de faturado por sessão e devolve a contagem por categoria (R6.4/B) e o total faturado das **realizadas**. Reutilizado pelas métricas do paciente e pelo relatório por local — mesma regra, filtro de origem diferente. Spec D7.1.
2. **Agrupamento sempre por `starts_at`** (data de início da sessão), no fuso `America/Sao_Paulo`, tanto nas métricas do paciente quanto no relatório por local. Não se usa `created_at` para o recorte temporal. Spec R6.4, R7.1 · D7.4.
3. **Taxas zero-safe.** Comparecimento = realizadas / (realizadas + faltas + canceladas-paciente); cancelamento = (canceladas-paciente + canceladas-terapeuta + canceladas-sem-autor) / total. Denominador zero ⇒ taxa 0 (sem divisão por zero). As taxas trafegam como fração 0..1; a UI formata em %. Spec R6.4.
4. **Faturado = soma dos débitos vinculados às sessões realizadas.** Para o paciente, casa `debt.session_id` com as realizadas via `debts.ListByPatient`; para o local, via novo `debts.ListBySessionIDs` sobre as sessões do recorte. Representa faturamento (das realizadas), não recebimento. Spec R7.2.
5. **Defaults de período.** Métricas do paciente: histórico completo (from/to ausentes). Relatório por local: mês atual (1º ao 1º dia do mês seguinte, fuso local). Spec D7.4.
6. **Exportação (CSV/impressão) fora do escopo mínimo** desta onda (R7.3); fica como melhoria futura. O relatório é legível na tela para repasse.

**Consequências.**
- Novo `SessionMetricsService` (motor + métricas do paciente + relatório por local); novos repositórios `sessions.ListByLocationRange` e `debts.ListBySessionIDs`.
- Novos endpoints `GET /v1/admin/patients/{id}/session-metrics` e `GET /v1/admin/locations/{id}/session-report`. Swagger atualizado.
- UI: bloco de KPIs de sessão na timeline do paciente; nova seção "Relatório de sessões por local" (seletor de local + navegação por mês + tabela de categorias + total + faturado), item no menu Gestão.
- Testes: unit do agregador (categorias incl. cancelada sem autor, taxas zero-safe, faturado só realizadas) + serviços; E2E com sessões variadas para ambos os endpoints.

---

## 2026-10-12 — Onda "patient-timeline" (Fase C): histórico do paciente e rastreabilidade de remarcação

**Contexto.** Spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md) R4. A tela base do paciente passará a ser uma timeline unificada; o backend precisa fornecer o feed já normalizado e registrar as remarcações, que hoje não são rastreáveis.

**Decisões.**
1. **Composição do histórico no servidor (opção B do design D4.4).** Novo `HistoryService` + endpoint `GET /v1/admin/patients/{id}/history` que agrega e normaliza os eventos (`{tipo, categoria, data, titulo, resumo, ref}`), ordenados desc, **derivados** das fontes existentes (sessões, notas, anamnese, débitos, pagamentos, arquivos, aprovação) + remarcações do audit. Preferido ao cálculo só no cliente pela testabilidade (E2E de contrato) e por centralizar a auditoria de leitura clínica. Tipos: `cadastro, aprovacao, sessao, remarcacao, registro, nota_livre, anamnese, debito, pagamento, arquivo`. Spec R4.1–R4.2 · D4.1/D4.4.
2. **Remarcação via audit `session.reschedule`.** `SessionService.Update`, ao alterar o horário de uma sessão `agendada`/`confirmada`, grava audit `session.reschedule` com `{session_id, from, to}` (datas, sem conteúdo clínico). A timeline deriva a entrada de remarcação desse log. Não se criou coluna/tabela nova para remarcação. Spec R4.3 · D4.3.
3. **Auditoria de leitura clínica na carga do histórico.** Como o feed agrega conteúdo clínico, cada carga de `history` registra um `AuditActionRead` sobre `patient_history`, com metadata **sem** conteúdo clínico (só `{patient_id, itens}`). O resumo textual exibido na UI não vai para metadados de auditoria. Spec R4.5.
4. **`AuditRepository.ListByEntity`.** Para derivar remarcações por sessão, o repositório de auditoria ganhou `ListByEntity(entity, entity_id)`.

**Consequências.**
- Novo endpoint `GET /v1/admin/patients/{id}/history`; novo `HistoryService`; nova ação de audit `session.reschedule` (constante já criada na Fase B); `AuditRepository.ListByEntity` + fakes de teste ajustados.
- Swagger, `docs/architecture.md` (§4.1.1/§4.1.2 + rota) e `docs/requirements.md` (§3.6 timeline) atualizados.
- Testes: unit do `HistoryService` (ordenação desc, todos os tipos, remarcação do audit, cancelamento com autor, sem conteúdo clínico em metadata); unit do `Update` (grava/não-grava reschedule); E2E de contrato do fluxo completo.

---

## 2026-10-12 — Onda "patient-timeline" (Fase B): controle de status da sessão

**Contexto.** Spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md) R6. A máquina de estados da sessão (`solicitada → agendada → {realizada, cancelada, falta}`) não tinha passo de **confirmação** e não distinguia **quem cancelou** (paciente × terapeuta), nem registrava quando/por quê. O terapeuta quer o ciclo de vida completo e base para métricas/relatórios.

**Decisões.**
1. **Opção B — status enxuto + metadados ortogonais.** Em vez de desdobrar o status por autor (`cancelada_paciente`, `cancelada_terapeuta`), o status continua único (`cancelada`) e o autor/motivo/datas são **campos** da sessão: `confirmed_at`, `canceled_by` (`paciente`|`terapeuta`), `cancel_reason` (texto livre opcional), `canceled_at`. Novo estado `confirmada` entre `agendada` e os desfechos. Transições: `agendada → confirmada|realizada|cancelada|falta`; `confirmada → realizada|cancelada|falta`. A confirmação é **opcional** (uma agendada ainda vai direto a realizada/falta/cancelada). Spec R6.1–R6.2 · D6.1.
2. **Rebuild da tabela `session` na migration 0004.** SQLite não altera `CHECK` por `ALTER`; incluir `confirmada` no `CHECK (status IN (...))` exige **recriar a tabela** (criar `session_new` com o CHECK atualizado + as 4 colunas novas, copiar os dados com metadados nulos, dropar/renomear, recriar os índices). Feito dentro da transação do framework de migrations, preservando dados, FKs e índices. Registros legados ficam com metadados nulos. Spec R6.3 · D6.2.
3. **Cancelamento exige autor; auditoria sem conteúdo clínico.** `SessionService.Cancel(ctx, id, by, reason)` recusa `by` ausente/inválido (HTTP 422) e grava os metadados; `Confirm` grava `confirmed_at`. As operações auditam `session.confirm` / `session.cancel` (metadata só `{canceled_by}` ou `{confirmed_at}` — o motivo NÃO entra em metadata, coerente com `§4.1`). `Finish`/`MarkNoShow` aceitam origem `confirmada` (via transições). Spec R6.2 · D6.3.
4. **Categoria de relatório `SessionCategory` (função pura).** Deriva de `status + canceled_by`: realizadas, faltas, canceladas-paciente, canceladas-terapeuta, canceladas-sem-autor (legado), agendadas/confirmadas, solicitadas. Reutilizada pelas métricas do paciente (F) e pelo relatório por local (F). Spec R6.4 · D6.4.

**Consequências.**
- Migration `0004_session_status_meta.sql` (rebuild idempotente no boot).
- Contrato: `sessionView` passa a expor `confirmed_at`/`canceled_by`/`cancel_reason`/`canceled_at` (quando houver); novo `POST /sessions/{id}/confirm`; `POST /sessions/{id}/cancel` passa a exigir corpo `{canceled_by, cancel_reason?}`. Novas ações de audit `session.confirm`/`session.cancel`/`session.reschedule`. Swagger atualizado.
- UI do painel da sessão: botão "Confirmar" (agendada), finalizar/falta disponíveis em agendada e confirmada, cancelamento com seletor de autor + motivo; `statusChip`/`Fmt.sessionStatus` ganham `confirmada`.
- **Breaking (interno):** a assinatura de `SessionService.Cancel` mudou de `Cancel(ctx, id)` para `Cancel(ctx, id, by, reason)`; o único chamador (handler admin) foi ajustado. O corpo do endpoint `/cancel` passou a ser obrigatório (autor) — fixtures E2E atualizadas.

---

## 2026-10-12 — Onda "patient-timeline" (Fase A): faturamento por sessão e ciclo por vencimento

**Contexto.** Terceira onda de evoluções (spec [`patient-timeline`](../.kiro/specs/patient-timeline/requirements.md)). O terapeuta quer que o faturamento por sessão seja automático e que os planos fechados cobrem a partir de um dia de vencimento, não com `+7 dias` arbitrário. Esta entrada cobre a **Fase A** (R1); as demais fases registram suas decisões aqui conforme forem fechando.

**Decisões.**
1. **`Bill` vira opt-out explícito.** Toda sessão `realizada` de paciente com plano por-sessão gera o débito **por padrão**. O flag `Bill` deixa de ser pré-requisito: `Bill == false` passa a significar "esta sessão não gera cobrança" (opt-out deliberado). No contrato HTTP, o campo `bill` em `POST /sessions/{id}/finish` virou **opcional** (`*bool`): ausente ⇒ cobra; `false` ⇒ opt-out. `SessionService.Finish` marca `Bill = true` quando o flag não é informado. Spec R1.3 · D1.2.
2. **Dia de vencimento no plano (`due_day`).** O `plan` ganha `due_day` (1–28, para evitar meses curtos; 0 = não aplicável a planos por sessão/social). Migration `0003_plan_due_day.sql` aditiva: default 1, e `due_day = 5` para os planos fechados pré-existentes (padrão coerente). Spec R1.5 · D1.1.
3. **Ciclo dispara a partir do vencimento; `due_date` real.** `CloseCycles` só gera o débito do período quando `ref` alcança o `due_day` do período (`ref >= vencimento`). O `due_date` do débito passa a ser a data real do vencimento (ano/mês do período + `due_day`), não mais `now + 7 dias`. Trimestral dispara pelo `due_day` do **1º mês** do trimestre; a idempotência por `cycle:<plan_id>:<período>` garante uma única cobrança por período, mesmo rodando o cron diário várias vezes. Spec R1.2, R1.5 · D1.3.
4. **Sem plano vigente ⇒ skip observável.** Finalizar uma sessão de paciente sem plano vigente não gera débito (sem base de valor) e registra um audit `debt_skip_no_plan` (sem conteúdo clínico), para o terapeuta perceber que faltou cadastrar o plano. Spec R1.2 · D1.2.

**Consequências.**
- Migration `0003_plan_due_day.sql` (aditiva, idempotente no boot).
- Contrato: `due_day` em `POST /v1/admin/plans` e no `planView`; `bill` opcional em `POST /sessions/{id}/finish`. Swagger atualizado.
- UI do plano: seletor de dia de vencimento (1–28) visível só nos planos fechados.
- Testes: unit de validação do `due_day`; unit de billing (opt-out, skip sem plano, gatilho antes/depois do vencimento, `due_date` real, trimestral pelo 1º mês); migration 0003 em base com/sem dados; E2E de geração por padrão e opt-out.
- Cron (`CloseCycles` diário) inalterado no agendamento; nada de geração preguiçosa nesta onda.

---

## 2026-10-12 — Onda "prontuario-ux": prontuário unificado, edição de sessão, WhatsApp, paleta e calendário fluido

**Contexto.** Pedido de produto do terapeuta focado em usabilidade do acompanhamento clínico e identidade visual. O detalhe do paciente fragmentava o conteúdo clínico em abas separadas (Anamnese, Sessões, Notas) e não permitia abrir uma sessão para editá-la nem registrar o relatório daquela sessão; o telefone era texto puro (sem link e sem padronização); o tema branco+verde/teal soava genérico; e a agenda semanal não acomodava bem um dia com ~10 pacientes. Detalhado na spec [`prontuario-ux`](../.kiro/specs/prontuario-ux/requirements.md).

**Decisões.**
1. **Prontuário unificado.** O detalhe do paciente ganha uma aba única "Prontuário" (timeline por data: sessões com seus registros + notas livres), substituindo as abas "Sessões" e "Notas". A "Anamnese" permanece separada (documento único). O ato de agendar continua na Agenda; o Prontuário trata do registro clínico. Reusa `domain.Note.SessionID` (já existente). Spec R1, R2.4.
2. **Edição de sessão (`PUT /v1/admin/sessions/{id}`).** Nova operação para editar data/hora, local e modalidade. Não troca o paciente. Bloqueada em estados terminais (realizada/cancelada/falta); auditada. A persistência (`SessionRepo.Update`) já cobria esses campos — sem mudança de schema. Spec R2.2.
3. **Edição de sessão NÃO reconcilia o Google Calendar nesta onda.** O `SessionCalendar` atual só expõe `CreateEvent`; implementar `UpdateEvent`/remoção remota amplia escopo e risco. A edição altera o registro local e a UI avisa o terapeuta para ajustar o convite no Google manualmente quando o horário mudar. `UpdateEvent` fica como melhoria futura.
4. **Registros vinculados à sessão.** A UI passa a enviar `session_id` ao criar nota a partir de uma sessão (contrato já existente em `POST /v1/admin/patients/{id}/notes`), correlacionando relatório ↔ sessão no Prontuário. Spec R2.3.
5. **Normalização de telefone no domínio.** `domain.NormalizePhone` canoniza para só dígitos com DDI `55`, aplicada no serviço de paciente (admin e portal), espelhando `NormalizeCPF`. O telefone vira link `https://wa.me/<numero>` (WhatsApp Web) no admin. Decisão: normalizar na entrada (não só a massa de testes), para o dado chegar correto. Spec R3.
6. **Nova paleta.** Azul royal como primária e lilás como acento, sobre fundos claros derivados e branco; cores de estado (ok/atenção/perigo) preservadas. Troca centralizada nos tokens `:root` de `app.css`. Spec R4.
7. **Calendário semanal fluido (dia cresce com a lotação), não grade por horas.** Escolhido o modelo em que a coluna do dia cresce conforme o número de sessões (fluido para ~10/dia), mantendo seg–sáb (sem domingo). A grade por eixo de horas (estilo Google com linha do tempo vertical) fica fora de escopo por custo/complexidade, dado o público leigo. Spec R5.

**Consequências.**
- Novo endpoint `PUT /v1/admin/sessions/{id}` (Swagger a atualizar); `sessionView` inalterado.
- `domain.NormalizePhone` + validação de plausibilidade do telefone; valor persistido de `phone` passa a ser canônico (comportamento, não shape).
- UI do admin: aba "Prontuário" (remove "Sessões"/"Notas"), painel de sessão com edição + registros, telefone como link WhatsApp, nova paleta, semana fluida.
- `scripts/seed.sh` passa a criar várias sessões no mesmo dia (exercita calendário lotado e links).
- Esta onda integra o mesmo PR grande para `main` que a `mvp-audit1` (push ao final, confirmado com o Fino).
- Execução planejada em fases (A–F) com commits por fase; ver [`tasks.md`](../.kiro/specs/prontuario-ux/tasks.md).

---

## 2026-09-06 — Padrão de documentação e política de branch por onda

**Contexto.** Os documentos estavam com nomes inconsistentes (`DEPLOY.md`, `DECISIONS.md` em maiúsculas; `requirements.md`/`architecture.md` em minúsculas) e espalhados entre a raiz e `docs/`. Faltava também um lugar para notas de versão legíveis e uma política clara de branch para trabalhos longos.

**Decisões.**
1. **Nomenclatura única de docs**: raiz contém apenas `README.md`; toda documentação técnica em `docs/` com nomes em `kebab-case` minúsculo. Movidos `DEPLOY.md` → `docs/deploy.md` e `DECISIONS.md` → `docs/decisions.md`.
2. **Release notes**: novo `docs/release-notes.md`, visual e orientado ao usuário, atualizado no fechamento de cada onda.
3. **Branch por onda de specs**: uma branch dedicada por conjunto grande de trabalho (não por task), com commits por fase e push único ao final.

**Consequências.** Links atualizados em `README.md`, `Makefile`, `scripts/deploy.sh` e nas specs. Política registrada em [`.kiro/steering/psicoman-processo-entrega.md`](../.kiro/steering/psicoman-processo-entrega.md).

---

## 2026-09-06 — Onda "mvp-audit1": auditoria de interface e segurança de acesso

**Contexto.** Após o MVP, uma auditoria de **requisitos × interface** revelou que várias funcionalidades existiam no backend sem tela usável (locais, origens, planos, custos/ROI, perfil do terapeuta, backup, auditoria) e que o portal do paciente expunha JSON cru e usava diálogos nativos do navegador — ruim para o público-alvo (usuários leigos). Além disso, o portal fica exposto na internet atrás do Pangolin sem controle de acesso, dependendo apenas do login social.

**Decisões.**
1. **Gate de aprovação de paciente (segurança).** Todo paciente que se auto-cadastra pelo portal nasce `pendente` e não acessa nenhum recurso até o terapeuta aprovar. Enforcement no servidor (middleware `ApprovalGate`, HTTP 403), não só na UI. Pacientes cadastrados pelo terapeuta nascem `aprovado`; vínculo por email não rebaixa quem já é aprovado. Detalhado na spec [`mvp-audit1`](../.kiro/specs/mvp-audit1/requirements.md) R1.
2. **Fechar os gaps de UI** identificados (locais, origem, planos, agendar sessão direto, custos/ROI, perfil, backup/restore, auditoria) — spec `mvp-audit1` R2–R9.
3. **Usabilidade para leigos** como requisito de primeira classe: portal sem JSON, fim de `window.prompt/confirm`, copy de login coerente, navegação clara, datas curtas, WCAG AA — spec `mvp-audit1` R10.
4. **Processo de entrega** formalizado em steering: toda mudança atualiza docs + testes no mesmo ciclo; commit ao fim de cada grupo de tasks e push só no fim da onda; decisões relevantes registradas aqui.

**Consequências.**
- Migration aditiva de `approval_status` em `patients` (pré-existentes → `aprovado`, idempotente).
- Novos endpoints: `GET /v1/portal/approval-status`; `GET /v1/admin/patients/pending`, `POST /v1/admin/patients/{id}/approve|reject`; `patientView` ganha `approval_status`.
- A suíte E2E do portal passa a exigir paciente aprovado nas rotas de recurso (ajuste de fixtures + casos 403/200).
- Execução planejada em fases (A–E) com commits por fase.

---

## 2026-09-06 — Telas de Agenda e Detalhe do Paciente no admin

**Contexto.** O painel admin só tinha lista/cadastro de pacientes, pedidos de agendamento e resumo financeiro. Clicar num paciente não abria nada (o item não tinha ação) e não existia visão de agenda — sendo essas as duas telas mais importantes para o dia a dia do terapeuta.

**Decisão.** Implementar, em SSR + Alpine (sem SPA, conforme `architecture.md §9`):
- **Agenda semanal** em destaque, de **segunda a sábado** (o terapeuta não atende domingo), com navegação por semana e sessões clicáveis. Consome `GET /v1/admin/sessions`.
- **Detalhe do paciente** com abas (dados, anamnese, sessões, notas, débitos/pagamentos/recibos, custos, arquivos), consumindo os endpoints já existentes por paciente.
- Store Alpine `nav` controla o paciente aberto; navegação por âncora mantida.

**Consequências.** Nenhuma mudança de backend; só UI (`internal/web`). Fluxo validado ponta a ponta em modo dev.

---

## 2026-09-06 — Markdown nos campos de texto e alinhamento cliente/servidor

**Contexto.** Requisito de que campos de texto livre (anamnese, notas, templates) sejam escritos em Markdown e exibidos formatados. O render do servidor (`internal/platform/markdown`) suportava só cabeçalhos, listas, negrito e itálico; o preview no admin precisava bater com o e-mail efetivamente enviado ao paciente.

**Decisão.** **Alinhar por cima**: estender o Markdown do servidor para também suportar **código inline**, **blocos de código cercados por ```** e **links** (apenas `http`/`https`/`mailto`, bloqueando `javascript:`), espelhado por um render client-side (`Md.toHtml` em `api.js`) com o mesmo subconjunto e escape anti-XSS. Editores no admin ganham alternância "Escrever/Visualizar".

**Consequências.** Preview no admin é fiel ao enviado. `internal/platform/markdown` ganhou testes para os novos casos (incluindo rejeição de `javascript:`). O subconjunto suportado é deliberadamente mínimo e deve ser mantido em paridade entre servidor e cliente.
