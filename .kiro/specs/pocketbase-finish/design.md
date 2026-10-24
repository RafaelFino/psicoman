# pocketbase-finish — Design

Como resolver os gaps de [`requirements.md`](./requirements.md), respeitando os invariantes da `pocketbase-core` (binário único; `domain`/`app`/`config` não importam o PocketBase; cifra na borda + DTO/`Reveal`; contrato `/v1` por ator; envelope PT-BR). Nada aqui reabre a arquitetura-base.

> **Status:** este é um documento de **plano**. Vários pontos dependem de decisão do Rafael (marcados **⟨decisão⟩**) antes da implementação.

---

## D1. Exposição no `/v1/app` (R1) — handlers finos sobre serviços existentes

A estratégia é a mesma já usada na `pocketbase-core`: handler fino em `internal/api/app` (ou `portal`) que chama o serviço do container e responde por **DTO explícito** no envelope. Os serviços já existem e estão testados — o trabalho é **superfície HTTP + DTO + E2E de contrato**, não regra nova.

| Gap | Rota | Serviço | Observações |
|-----|------|---------|-------------|
| R1.1 feed do dia | `GET /v1/app/day-feed?window=` | `DayFeedService` | janela hoje/7d/recentes; sem clínico |
| R1.2 história | `GET /v1/app/patients/{id}/history` | `HistoryService` | auditar leitura (`AuditActionRead`), sem clínico em metadados |
| R1.3 métricas | `GET /v1/app/patients/{id}/session-metrics?de=&ate=` | `SessionMetricsService` | recorte opcional por período |
| R1.4 calendário | `GET /v1/app/sessions?patient_id=&from=&to=` | `SessionService` | `ListByRange`/`ListByPatient` já existem |
| R1.5 relatório por local | `GET /v1/app/locations/{id}/session-report?de=&ate=` | `CostService`/`SessionMetricsService` | confirmar qual serviço agrega (ver código) |
| R1.6 custos/ROI | CRUD `cost_category`/`cost_item`/`session_cost` + `GET .../reports/costs` | `CostService` | ROI já exposto; faltam CRUD e relatório de custo |
| R1.7 locais/disp./origens | CRUD `/v1/app/locations`, `/availability`, `/v1/app/origins` | repos existentes (sem serviço dedicado) | pode exigir um `LocationService`/`OriginService` enxuto |
| R1.8 pedidos de agendamento | `POST /v1/portal/appointment-requests`, `GET /v1/app/appointment-requests`, `POST /v1/app/appointment-requests/{id}/confirm` | `AppointmentRepository` + `SessionService.Schedule` | o hook de aprovação já reforça; confirmar cria a sessão |
| R1.9 planos/pagamentos | CRUD `/v1/app/patients/{id}/plans`; `POST /v1/app/debts/{id}/pay` (existe) + anexo de comprovante | `BillingService`/`PaymentService` + GED | comprovante usa o `GEDBlobStore` da Fase 6 |

**DTOs:** nenhum DTO do portal inclui clínico; o `history`/prontuário só sai em claro pelo ator app via `Reveal()` (D6.3 da `pocketbase-core`). Swagger atualizado a cada rota.

**⟨decisão R1⟩ Recorte/priorização.** R1 é a maior parte da onda. Proposta de fatiar em sub-fases por valor:
1. **Núcleo utilizável** (R1.7 locais + R1.9 planos → o débito por sessão passa a funcionar ponta a ponta via API) — maior destravamento.
2. **Telas iniciais** (R1.1 feed + R1.2 história + R1.3 métricas + R1.4 calendário).
3. **Operação financeira/custos** (R1.6 custos/ROI completo + R1.5 relatório por local).
4. **Agendamento pelo portal** (R1.8).

Recomendo fazer nessa ordem. O Rafael decide se tudo entra nesta onda ou se 3/4 viram onda seguinte.

---

## D2. OAuth 3-legged + `google_token` cifrado (R2)

- **Adapter `google_token`** em `platform/pb`: porta `GoogleTokenRepository` (`app`) com `Save`/`Load`; o campo `token` guarda o **envelope cifrado** (reusa `domain.Cipher`/`SealFor`/`OpenInto`, como o prontuário). `scopes` (JSON) e `expiry` (ISO) em claro.
- **Fluxo de autorização:** `GET /v1/admin/google/authorize` monta a URL de consentimento (escopos `calendar` + `drive.file`, `access_type=offline`, `prompt=consent`) e redireciona; `GET /v1/admin/google/callback` recebe o `code`, troca pelo refresh token (via `golang.org/x/oauth2`, já dependência) e persiste cifrado. Rotas no ator **admin** (operação), atrás do Pangolin.
- **Boot:** `wireCalendar`/`wireDriveSync` passam a carregar o refresh token do `google_token` (fallback para o env `PSICOMAN_GOOGLE_REFRESH_TOKEN` só em dev, **⟨decisão R2⟩**). Renovação automática pelo `oauth2.TokenSource`.
- **Reautorização necessária:** um estado (em memória + flag derivada do `google_token` ausente/expirado sem refresh) exposto em `/readyz` (degraded) e por uma rota admin de status. Falha de renovação é logada, não derruba (`psicoman-google-api`: "sem panic, degraded").
- **Testes:** adapter round-trip do token cifrado (não-vazamento); fluxo de callback com servidor OAuth fake; boot lê token do repo; estado degradado quando ausente. Nenhum teste toca o Google real.

---

## D3. Perfil do terapeuta (R3)

- **Porta + adapter + serviço** para `therapist_profile` e `therapist_platform_link` (padrão dos demais contexts). Como é 1:1 por instância, o serviço expõe `Get`/`Upsert` (singleton lógico) + gestão de links e locais associados.
- **Rotas `/v1/app`:** `GET/PUT /v1/app/therapist` (perfil), `POST/DELETE /v1/app/therapist/platform-links`. Dados públicos ao portal via `GET /v1/portal/therapist` (só `PublicView()`).
- **PDF de cobrança:** `InvoiceService.compose` recebe o perfil (injeção da porta `therapist`) e volta a imprimir nome/CRP quando configurado — removendo o "fica de fora" atual.
- **Testes:** unit do serviço (upsert idempotente 1:1); E2E do contrato; E2E da cobrança mostrando o cabeçalho; portal vê só público.

---

## D4. Verificação de email do portal (R4) — **⟨decisão⟩**

Duas opções:
- **A) Manter sem verificação (recomendo):** o gate de aprovação do terapeuta já é o controle de acesso efetivo; a credencial nasce `verified=true`. Simples, sem dependência de transporte de email. O risco de email falso é mitigado pela aprovação humana.
- **B) Ligar verificação de email:** usa o fluxo nativo do PocketBase (envia token por email), exige configurar um **transporte SMTP** (o PocketBase tem suporte nativo; não é o Gmail da app). Acrescenta uma etapa antes da aprovação.

Recomendação: **A**, salvo se o Rafael quiser a verificação por política. Se **B**, entra a config de SMTP e o fluxo de confirmação + E2E.

---

## D5. Observabilidade (R5)

- `/readyz`: checa cifra montada, DB acessível (ping simples) e o estado do OAuth Google (ok|degraded por reautorização). `/livez`: 200 fixo (processo vivo).
- **Métricas:** **⟨decisão R5⟩** — mínimo viável é o logging de timing já existente (`httpx.Timing`/`Logging`); um endpoint Prometheus é desejável mas pode ser adiado. Proposta: entregar `/readyz`+`/livez` nesta onda e registrar métricas como item explícito (fazer agora ou adiar).

---

## D6. Docs de deploy (R6)

- Reescrever `docs/deploy.md` e `docs/stack-local.md` para o binário único: `bin/psicoman serve --http 127.0.0.1:8090 --dir ./pb_data` atrás do Pangolin; `config.yaml` a partir do `config.example.yaml`; autorização OAuth (D2); um `systemd` unit; atualização com backup + rebuild + migrations no boot. Remover as notas de "desatualizado".
- Depende de D2 (o guia descreve a autorização real) — fazer **depois** de R2, ou descrever o fluxo-alvo e ajustar ao final.

---

## D7. Ordem sugerida das fases

Respeitando dependências: R3 (perfil) e R2 (OAuth) são independentes entre si e destravam qualidade (cobrança completa; Google sem env). R1 é o grosso (fatiado por D1). R5 depende de R2 para o `/readyz` refletir o Google. R6 depende de R2/R5 para descrever a operação real. Ver [`tasks.md`](./tasks.md).

---

## Pontos que exigem decisão do Rafael (resumo)

1. **⟨R1⟩** Recorte: tudo nesta onda ou fatiar (3/4 para onda seguinte)? Ordem sugerida em D1.
2. **⟨R2⟩** Env `PSICOMAN_GOOGLE_REFRESH_TOKEN` continua como fallback de dev depois do fluxo real?
3. **⟨R4⟩** Verificação de email do portal: manter desligada (recomendado) ou ligar (exige SMTP)?
4. **⟨R5⟩** Métricas: endpoint dedicado nesta onda ou adiar (só `/readyz`+`/livez` agora)?
