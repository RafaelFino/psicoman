# pocketbase-finish — Requisitos

Onda de **fechamento de pendências e gaps** deixados pela onda [`pocketbase-core`](../pocketbase-core/). A `pocketbase-core` reconstruiu a base técnica (binário único com PocketBase, domínio rico, cifra por campo, contrato `/v1` por ator, Calendar síncrono, Drive DR, Gemini, portal com auth) e **portou as regras de negócio** das ondas anteriores para o domínio rico — mas **nem toda feature de produto já implementada no serviço foi exposta no contrato `/v1`**, e alguns pontos ficaram deliberadamente como seam/fora de escopo. Esta onda resolve isso.

> Responde **o quê**/**por quê**. O **como** fica em [`design.md`](./design.md); o plano em [`tasks.md`](./tasks.md). Fonte dos gaps: varredura do código (TODOs, seams, "fase posterior", "fora de escopo") + notas de desatualizado nos docs, consolidada em [`../../../docs/decisions.md`](../../../docs/decisions.md).

Princípios herdados (não reabrir): binário único; regra de dependência (`domain`/`app`/`config` não importam o PocketBase; só `platform/pb` importa); cifra por campo `SealedText` na borda + DTO/`Reveal()`; contrato `/v1` por ator com envelope PT-BR; segredos nunca em log/repo; ULID, centavos `int64`, `America/Sao_Paulo`.

---

## R1. Exposição no `/v1/app` das features de produto já implementadas

**Contexto.** Os serviços `DayFeedService`, `HistoryService`, `SessionMetricsService` e `CostService` estão montados no container (`internal/api/service`), com regras portadas e testadas em unit, mas **sem handler nem rota** no `/v1`. São features que o produto já tinha nas ondas `prontuario-ux`/`patient-timeline` e que hoje estão inacessíveis pela API nova. Também faltam rotas para recursos que têm repositório/porta mas nenhuma superfície HTTP (locais e disponibilidades, origens, pedidos de agendamento, planos, pagamentos com comprovante).

- **R1.1 Feed do dia do terapeuta** (`patient-timeline` R8): `GET /v1/app/day-feed` com janela ajustável (hoje / próximos 7 dias / recentes), eventos agrupados por faixa temporal, sem conteúdo clínico.
- **R1.2 Timeline/história do paciente** (`patient-timeline` R4): `GET /v1/app/patients/{id}/history` — feed cronológico composto no servidor (cadastro, sessões, remarcações, registros clínicos, débitos, pagamentos, arquivos), com a leitura auditada (`AuditActionRead`) e **sem** conteúdo clínico em metadados.
- **R1.3 Métricas de sessão do paciente** (`patient-timeline` R6.4–R6.5): `GET /v1/app/patients/{id}/session-metrics` (comparecimento, cancelamento, quebra por categoria), com recorte opcional por período.
- **R1.4 Calendário mensal do paciente** (`patient-timeline` R2): `GET /v1/app/sessions?patient_id=&from=&to=` (listagem por paciente e intervalo), base da visão de calendário.
- **R1.5 Relatório de sessões por local** (`patient-timeline` R7): `GET /v1/app/locations/{id}/session-report` (agregação por categoria + faturado das realizadas no período).
- **R1.6 Custos e ROI** (`requirements §3.5`): CRUD de categorias/itens de custo e custo de sessão, e o relatório de custos — hoje só `GET /v1/app/reports/roi` existe. Expor o que o `CostService` já faz.
- **R1.7 Locais, disponibilidades e origens**: CRUD de `location` + `availability` e de `origin` (base da agenda ofertada e do ROI), que têm repositório mas nenhuma rota.
- **R1.8 Pedidos de agendamento** (`appointment_request`): o fluxo do paciente criar um pedido e o terapeuta ver/confirmar a fila de pendências — a coleção e o hook de aprovação existem, mas não há rota de criação (portal) nem de listagem/confirmação (app).
- **R1.9 Planos e pagamentos**: CRUD de `plan` por paciente e registro de `payment` com anexo de comprovante no GED — o débito por sessão depende de um plano vigente, hoje não cadastrável pela API.

**Aceite.** Cada feature acima responde no envelope `/v1`, coberta por E2E de contrato; nenhuma resposta do portal inclui clínico; a leitura de prontuário/história segue auditada.

**Decisão necessária (Rafael):** priorização/recorte — R1 é grande. Candidato a quebrar em sub-ondas (ver `design.md`). O conjunto mínimo para "produto utilizável de ponta a ponta" é R1.7 + R1.9 (locais + planos, para o débito por sessão funcionar via API) + R1.1/R1.2 (feed e história, as telas iniciais).

---

## R2. OAuth 3-legged real do Google + token cifrado

**Contexto.** Hoje o refresh token do Google vem de uma env var (`PSICOMAN_GOOGLE_REFRESH_TOKEN`, seam em `wiring.go`), e a coleção `google_token` (criada na migração 0030, com `token` cifrado + `scopes` + `expiry`) **não tem adapter**. Calendar e Drive degradam sem o token, mas não há fluxo para o terapeuta **autorizar** a instância.

- **R2.1** Fluxo de autorização OAuth 3-legged (conta pessoal, sem domain-wide delegation): o terapeuta autoriza uma vez; o sistema recebe o callback, troca o code pelo refresh token e o **persiste cifrado** na coleção `google_token` (via `SealedText`/`Cipher`, como o prontuário).
- **R2.2** Carregar o refresh token do `google_token` no boot (substituindo o seam de env), com renovação automática do access token em runtime.
- **R2.3** Falha ao renovar não derruba o processo: sinaliza **"reautorização necessária"** — exposto em `/readyz` (degraded) e para a UI do admin —, sem panic.
- **R2.4** Escopos mínimos mantidos (`calendar`, `drive.file`); segredos (client secret, refresh token) nunca em log nem em resposta.

**Aceite.** O terapeuta autoriza pela UI/rota; o token fica cifrado em `google_token`; o Calendar/Drive passam a funcionar sem env var; a perda do token vira estado degradado observável, sem queda.

**Decisão necessária (Rafael):** onde expor o início do fluxo (`/v1/admin/google/authorize` + callback) e se o env var seam continua como fallback de dev.

---

## R3. Perfil do terapeuta no domínio rico + rotas

**Contexto.** O agregado `domain/therapist` (`TherapistProfile`, `TherapistPlatformLink`) e a coleção `therapist_profile` existem, mas **sem porta, serviço e rotas**. O PDF de cobrança omite o cabeçalho do terapeuta (nome/CRP) por isso (`invoice_service.go compose`).

- **R3.1** Porta + adapter `platform/pb` + serviço para `therapist_profile` e `therapist_platform_link` (1 perfil por instância).
- **R3.2** Rotas `/v1/app`: ler/editar o perfil, gerenciar locais associados e links de plataforma (com vínculo opcional a `origin`).
- **R3.3** O PDF de cobrança volta a incluir o **cabeçalho do terapeuta** (nome/CRP) quando o perfil está configurado.
- **R3.4** Dados públicos do perfil (nome/foto/bio) disponíveis ao portal, conforme `PublicView()`.

**Aceite.** O terapeuta cadastra/edita o próprio perfil pela API; a cobrança em PDF mostra o cabeçalho; o portal lê só os dados públicos.

---

## R4. Verificação de email do portal (decisão pendente)

**Contexto.** Na Fase 9, a credencial `patient_user` nasce com `verified=true` — o controle de acesso é o **gate de aprovação** do terapeuta, não a verificação de email. Isso é suficiente para o modelo atual, mas foi marcado como fora de escopo.

- **R4.1 (a decidir)** Se vale introduzir verificação de email (fluxo nativo do PocketBase: envio de token por email) **antes** da aprovação do terapeuta, ou se o gate de aprovação já cobre o risco e a verificação fica dispensada.

**Decisão necessária (Rafael):** manter sem verificação (gate basta) **ou** ligar a verificação de email. Recomendação em `design.md`. Sem envio de email configurado na arquitetura (Gmail fora de escopo), ligar verificação exige definir o transporte (SMTP do PocketBase) — avaliar custo/benefício.

---

## R5. Observabilidade: readiness/liveness/métricas

**Contexto.** Os requisitos pedem health/readiness/liveness e métricas; hoje só `GET /v1/health` está montado. O estado "reautorização Google necessária" (R2.3) precisa de um `/readyz` que o reflita.

- **R5.1** `/readyz` (readiness: cifra ok, DB acessível, estado do OAuth Google) e `/livez` (liveness).
- **R5.2** Métricas básicas (ao menos contagem/latência por rota já logada), no mínimo um endpoint de métricas ou a decisão explícita de adiá-las.

**Aceite.** `/readyz` reflete o estado real (incl. degradação do Google); `/livez` responde; a operação consegue monitorar a instância.

---

## R6. Reescrita operacional dos docs de deploy

**Contexto.** `docs/deploy.md` e `docs/stack-local.md` descrevem o modelo **antigo** (dois binários, Gmail, `dev_mode`, portas 8080/8081) e estão com nota de "desatualizado". Precisam ser reescritos para o binário único.

- **R6.1** `docs/deploy.md`: instalação/operação de **um binário** (`serve --http --dir`) atrás do Pangolin, `config.yaml` (sem segredos no repo), credenciais Google (`calendar` + `drive.file`, sem Gmail), autorização OAuth (R2), systemd de um serviço, atualização segura.
- **R6.2** `docs/stack-local.md`: stack local do binário único (Docker) para teste/navegabilidade.
- **R6.3** Remover as notas de "desatualizado" ao concluir.

**Aceite.** Um operador consegue instalar e atualizar a instância seguindo o guia, no modelo atual; a stack local sobe o binário único.

---

## R7. Definição de pronto (herdada de `psicoman-processo-entrega`)

Toda mudança de comportamento fecha com: código verde (`make check`), docs atualizadas no mesmo ciclo (requirements/architecture/README/Swagger/decisions conforme o caso), e testes cobrindo (regra nova → unit; fluxo/contrato → E2E; segurança/acesso → E2E permitido **e** negado; cripto → round-trip + não-vazamento). Trabalho em branch da onda; commit por fase; push/PR só no fim e com OK do Rafael.
