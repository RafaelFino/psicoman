# Psicoman — Documento de Design
# Psicoman — Arquitetura

Design técnico da plataforma (arquitetura `pocketbase-core`). Complementa [`requirements.md`](./requirements.md) (requisitos) e as decisões em [`decisions.md`](./decisions.md). O plano de implementação da onda está em [`../.kiro/specs/pocketbase-core/`](../.kiro/specs/pocketbase-core/).

> **Arquitetura vigente.** Este documento descreve a arquitetura da onda `pocketbase-core` (binário único com PocketBase embarcado, domínio rico por bounded context, adapter `platform/pb`, cifra por campo `SealedText`, contrato `/v1` por ator, config YAML, Calendar síncrono, Drive como fallback/DR, Gemini + índice de tags, portal com auth própria). A decisão-base está na ADR `2026-10-21` de [`decisions.md`](./decisions.md); o modelo de dados canônico (MER / D4b) e os refinamentos de design em [`../.kiro/specs/pocketbase-core/design.md`](../.kiro/specs/pocketbase-core/design.md).

---

## 0. Visão geral e camadas

A instância é **um binário Go** (`cmd/psicoman`) com o **PocketBase embarcado como framework** (`pocketbase.New()` + registro das rotas `/v1/*` no hook `app.OnServe()`): SQLite relacional (WAL), servidor HTTP, file storage, auth/OAuth2, migrations e admin UI nativa (`/_/`) vêm de dentro dele. Roda sempre atrás do Pangolin. As subseções abaixo detalham cada camada entregue pela onda. Fonte do modelo de dados: D4b (MER canônico) em [`../.kiro/specs/pocketbase-core/design.md`](../.kiro/specs/pocketbase-core/design.md).

### 0.1 Camadas e regra de dependência

```
internal/domain/<context>   núcleo de regras (agregados ricos) — não importa PB, app, api
      ↑ (interfaces de porta tipadas contra o domínio)
internal/app                serviços enxutos + portas de repositório/integração
      ↑
internal/platform/pb        adapter (Fase 3) — único a importar o PocketBase
```

- **Domínio puro.** `internal/domain/*` só pode acoplar-se a `internal/platform/{ulid,clock}` e aos tipos transversais `domain.Cipher`/`domain.SealedText`. **Não** importa o PocketBase, `platform/pb`, `app` nem `api`. Imposto pelo teste de arquitetura `internal/archtest/arch_test.go` (`TestDomainNaoImportaPocketBase`), parte do gate.
- **Serviços enxutos.** `internal/app` orquestra os agregados e depende apenas das **portas** (interfaces), nunca de uma lib externa concreta. A lógica de negócio vive no agregado; o serviço só coordena I/O, auditoria e disparo de hooks.
- **Portas em `internal/app`.** As interfaces de repositório e de integração ficam em `internal/app` (não no domínio), junto do serviço que as consome (`internal/app/ports.go`). Mantém o domínio como núcleo puro, sem portas órfãs. O adapter `platform/pb` (Fase 3) implementa essas portas.

### 0.2 Bounded contexts

Seis contexts, cada um um pacote Go com seus agregados, validação, transições e funções puras:

| Context (`internal/domain/…`) | Agregados / tipos principais | Responsabilidade |
|---|---|---|
| `patient` | `Patient`, `Origin` | Cadastro, telefone/CPF canônico, gate de aprovação (`pendente`/`aprovado`/`rejeitado`), emissão de recibo. |
| `scheduling` | `Session`, `Location`, `Availability`, `AppointmentRequest` | Ciclo de vida da sessão (transições, confirmação, cancelamento com autor/motivo, remarcação) e `SessionCategory`. **Kernel compartilhado:** dono das constantes de `modalidade`, `período` e categoria de sessão. |
| `billing` | `Contract`, `Plan`, `Debt`, `Payment` | Débito por sessão (opt-out `bill`), fechamento de ciclo por `due_day` com idempotência (`session_id` ou `plan_id+billing_period`), `atendimento_social` nunca gera débito, status de pagamento (`aberto`/`parcial`/`pago`). |
| `costs` | `CostCategory`, `CostItem`, `SessionCost`, ROI | Custo direto/rateado por sessão, rateio por período, ROI por origem. |
| `clinical` | `Anamnesis`, `Note`, `AISummary`, `TagIndex`, timeline, métricas | Prontuário com campos clínicos em `SealedText`; timeline/métricas derivadas; tags do `tag_index` em **claro** (indexáveis), corpo cifrado. |
| `therapist` | `TherapistProfile`, `TherapistPlatformLink` | Perfil do terapeuta e links de plataformas (reusa `origin`). |

**Dependências entre contexts (unidirecionais e explícitas):**

- `scheduling` é o **kernel compartilhado**: não importa nenhum context irmão.
- `costs → scheduling` — para as constantes de período (`PeriodDiario/Mensal/Anual`) usadas no rateio.
- `clinical → scheduling` — o motor de métricas consome `scheduling.SessionCategory`.
- `clinical → billing` — a timeline referencia débitos/pagamentos do paciente.
- Onde um serviço precisa cruzar contexts sem criar import (ex.: finalizar sessão → gerar débito), a orquestração vive em `internal/app`, passando dados primitivos (IDs, valores, flags) em vez de importar o pacote irmão.

Transversais no pacote raiz `internal/domain`: `AuditLog` (`audit.go`), `GEDFile` (`ged.go`), além de `Cipher`/`SealedText` (Fase 1). Mantidos na raiz por não carregarem regra de um context específico.

### 0.3 Serviços de aplicação (`internal/app`)

Serviços enxutos, um por área, recebendo as portas por injeção (construtores `New…Service`) e um `clock.Clock` injetável para testes determinísticos: `SessionService`, `BillingService`, `PaymentService`, `CostService`, `ReportService`, `SessionMetricsService` (via `metrics_service.go`), `HistoryService`, `DayFeedService`, `PatientService`, `AuditService`. As regras de negócio maduras das ondas anteriores foram **portadas** (não reinventadas) para os agregados + serviços, com paridade de comportamento verificada por testes unitários (feliz + borda) usando fakes das portas.

### 0.4 Invariantes de domínio preservados

ULID em toda entidade (`internal/platform/ulid`), dinheiro em `int64` centavos BRL, instantes em `America/Sao_Paulo` via `internal/platform/clock`, erros ao usuário em PT-BR (sentinelas `ErrNotFound`/`ErrConflict`/`ErrValidation` em `internal/app/errors.go`), `context.Context` como 1º parâmetro de toda porta/serviço com I/O, e idempotência de efeito financeiro por chave única.

### 0.5 Coleções/migrations e adapter `platform/pb`

A Fase 3 materializa a camada de persistência: as **coleções do PocketBase** derivadas do MER canônico (D4b) e o **adapter** `internal/platform/pb`, único pacote que importa o PocketBase e que implementa as portas de `internal/app/ports.go`. O domínio continua limpo (archtest verde; `go list -deps ./internal/domain/...` sem `pocketbase`/`platform/pb`).

**Coleções e migrations.** As coleções vivem no subpacote `internal/platform/pb/migrations`, um arquivo por grupo do MER, cada um registrado via `migrations.Register(up, down, nome)` em `init()` e aplicado no boot por `RunAllMigrations()` (idempotente: reexecução é no-op). `cmd/psicoman/main.go` faz o blank import do subpacote. Os três grupos:

- `0010_patient_scheduling` — `origin`, `location`, `patient`, `availability`, `appointment_request`, `session`.
- `0020_billing_costs` — `cost_category`, `cost_item`, `contract`, `plan`, `debt`, `payment`, `session_cost` (ciclos `contract↔plan` e `session→contract/plan` resolvidos por patch de relation após ambas as coleções existirem).
- `0030_clinical_therapist_transversais` — `anamnesis`, `note`, `ai_summary`, `tag_index`, `therapist_profile`, `therapist_platform_link`, `ged_file`, `audit_log`, `google_token`.

**Convenções do schema (psicoman-sqlite.md):**

- **`id` ULID custom.** Em cada coleção o campo `id` default do PocketBase (15 chars `[a-z0-9]`, autogerado) é substituído por um `TextField{PrimaryKey, System, Required, Min:26, Max:26, Pattern:"^[0-9A-HJKMNP-TV-Z]{26}$"}` **sem** `AutogeneratePattern`. O PB não autogera; o adapter grava o ULID do agregado via `rec.Set("id", agg.ID)`. `initIdField` respeita o `id` pré-existente.
- **Timestamps TEXT ISO-8601.** `created_at`/`updated_at`/`deleted_at` são `TextField` serializados pela aplicação (`clock.Format` = RFC3339 em America/Sao_Paulo), nunca `AutodateField` nem `DEFAULT CURRENT_TIMESTAMP` (que usa UTC). `deleted_at` vazio = ativo.
- **Dinheiro** em `NumberField{OnlyInt:true}` (centavos).
- **Constraints.** UNIQUE `email` e UNIQUE parcial `cpf` em `patient`; UNIQUE `idempotency_key` em `debt` (idempotência de efeito financeiro); UNIQUE `session_id` em `session_cost` (1:1); UNIQUE `patient_id` em `anamnesis` (1:1).

**Adapter (um repositório por porta).** Cada repositório é um struct com um `core.App` (e um `domain.Cipher` nos que têm campo clínico). Mapeia agregado↔`core.Record` via `rec.Set(...)` (gravação) e `rec.GetString/GetInt64/GetBool` (leitura), propagando `context.Context` por `SaveWithContext`/`DeleteWithContext`. Erros do PocketBase são traduzidos para os sentinelas de `internal/app`: `sql.ErrNoRows → ErrNotFound`; violação de UNIQUE → `ErrConflict`. Soft-delete seta `deleted_at` e as listagens filtram `deleted_at = ''`. Asserts de compilação (`var _ app.XRepository = (*xRepo)(nil)`) garantem a conformidade com as portas.

**Cifra na borda (D6.3).** Apenas o `ProntuarioRepository` (coleções `anamnesis`/`note`) cifra campo clínico: ao gravar, `env, _ := domain.SealFor(ctx, cipher, agg.Content); rec.Set("conteudo", env)`; ao ler, `agg.Content, _ = domain.OpenInto(ctx, cipher, rec.GetString("conteudo"))`. O banco guarda **somente** o envelope (`v1.aes-256-gcm.id_da_chave.nonce.ciphertext`); o claro nunca é escrito (nunca `rec.Set("conteudo", agg.Content.Reveal())`). As colunas `ai_summary.resumo` e `google_token.token` também guardam envelope, mas ainda não têm adapter nesta fase.

### 0.6 Contrato `/v1` por ator

A Fase 4 expõe o **contrato HTTP público** `/v1/*`, segregado por ator para o Pangolin dividir por prefixo (D2/D7). Pacotes em `internal/api`: `httpx` (transversal: envelope, erros, chain), `app`/`portal`/`admin` (handlers por ator, nenhum importa o outro), `swagger` (OpenAPI do `/v1`), `service` (container `Services` montado pelo wiring). O wiring entra por `api.Register(app)`: monta repos+cipher+serviços dentro do `OnServe` e registra os grupos.

- **Atores e prefixos.** `/v1/app/*` (terapeuta — único que lê clínico em claro), `/v1/portal/*` (paciente — nunca recebe clínico), `/v1/admin/*` (operação). `/v1/health` e `/v1/swagger[.json]` ficam na raiz do `/v1`.
- **Envelope.** Toda resposta usa `{ message, elapsed_ms, data, error }` (ordem é contrato, R7.2; garantida por struct, nunca `map`). `message` sempre PT-BR; sucesso → `error:null`, falha → `data:null`. O id de correlação vai no header `X-Request-Id`, não no corpo.
- **Middleware chain** (grupo `/v1`): `recover → request-id → timing → logging`; a **auth** fica no subgrupo de cada ator (stub no-op na Fase 4; Pangolin/gate na Fase 5).
- **Mapeamento erro→HTTP** (via `errors.Is`): `ErrNotFound`→404 `nao_encontrado`; `ErrConflict`→409 `conflito`; `ErrValidation`→422 `validacao`; corpo malformado→400 `requisicao_invalida` (helper `FailBadRequest`, antes de `Fail`, não cai no 500); demais→500 `erro_interno` (detalhe só no log). A `message` de 4xx é sempre a frase PT-BR curta do serviço, truncada a 300 chars.
- **DTOs por ator (não-vazamento, D6.3/R6.3).** Nenhum DTO embute `domain.SealedText`. O ator `app` revela o clínico em claro via `.Reveal()` (só nos mappers de `internal/api/app`); nenhum DTO do `portal` tem campo clínico (`internal/api/portal` não importa `internal/domain/clinical` nem referencia `.Reveal(`). Dinheiro em `*_cents int64`; tempo via `httpx.FmtTime` (instante zero → `""`).
- **Prontuário.** Novo `app.ProntuarioService` (gera ULID/tempo, monta `SealedText`, valida) serve `POST /v1/app/patients/{id}/notes` e `PUT /v1/app/patients/{id}/anamnesis` (upsert 1:1); `GET /v1/app/patients/{id}` devolve o prontuário legível (anamnese opcional + notas).
- **REST automática trancada (D13).** As coleções do MER têm regras nulas → o PocketBase responde **403** a `/api/collections/.../records` para não-superuser; mesmo um superuser lê o campo clínico como envelope cifrado, nunca o claro. O contrato público é **exclusivamente** `/v1/*` (sem migration nova: só documentação + teste de contrato).
- **Config de cifra no boot.** `buildServices` lê a chave de `PSICOMAN_CIPHER_KEY_ID` + `PSICOMAN_CIPHER_KEYS` (`id:base64,...`); ausência/inválida aborta o boot (fail-closed). Não subir o servidor sem cifra.

### 0.7 Auth atrás do Pangolin e gate de aprovação

A Fase 5 substitui os stubs de auth no-op da chain (`internal/api/httpx/chain.go`) por controle real, mantendo a segregação por ator da Seção 0.6. São três peças coesas de segurança.

- **Header de identidade do Pangolin (`/v1/app/*` e `/v1/admin/*`).** O gateway Pangolin autentica na borda e injeta a identidade do usuário num header HTTP. O app exige apenas a **presença** desse header: ausente/vazio → **401 `nao_autenticado`** (envelope PT-BR). O nome do header é configurável pela env `PSICOMAN_PANGOLIN_IDENTITY_HEADER`, com **default `X-Pangolin-User`** — diferente da cifra (que aborta o boot se ausente), aqui há fallback porque o nome de um cabeçalho não é segredo. O middleware (`httpx.RequirePangolinIdentity`, construído com o nome resolvido no boot) grava a identidade no store (`httpx.ActorIdentity`) como **preparação de auditoria** — ainda sem consumidor nesta fase. A política fina (admin mais restrito que app) é responsabilidade do Pangolin na borda, não do código.
- **Admin UI nativa `/_/` e REST `/api/*` intactas.** Nenhum middleware do Pangolin incide sobre `/_/` (Dashboard com auth própria de superuser, segundo fator) nem sobre `/api/*` (REST automática já trancada por regra nil → **403** sem superuser, não 401). O middleware de header só cobre os grupos `/v1/app/*` e `/v1/admin/*`.
- **Gate de aprovação do portal (`/v1/portal/*`).** Middleware do grupo (`portal.RequireApproved`) resolve o paciente e **nega 403 `acesso_negado`** se `approval_status != "aprovado"`, EXCETO `POST /v1/portal/register` e `GET /v1/portal/me` (comparação por **path exato**, acessíveis a pendente/rejeitado para o paciente se cadastrar e ver o próprio estado). A identidade do paciente vem de um **seam** (`httpx.PortalEmail`, header `X-Portal-Email`); a auth real do paciente (sessão/`core.NewAuthCollection`) fica para a **Fase 9** — o seam é trocável sem mexer no gate.
- **Hook de reforço (defesa em profundidade).** Hook model-level (`OnRecordCreate`/`OnRecordUpdate`) em `internal/api/hooks.go` barra a escrita de `appointment_request` cujo paciente dono não esteja aprovado — por **qualquer** caminho, inclusive o adapter `platform/pb` (`app.Save`), que não passa pela chain HTTP. Escopo deliberadamente reduzido a `appointment_request` (única escrita originável pelo paciente no portal e com `patient_id` direto); `payment`/`ged_file`/`patient`/colunas clínicas ficam fora (ver `docs/decisions.md`). Invariante: agendar exige paciente aprovado, venha de quem vier.
- **Regra de dependência.** `httpx` (middleware de header, seams) importa só `core`+stdlib; o gate e o hook vivem em `internal/api`/`internal/api/portal`, que podem importar `app`/`service`/`pb`/`pocketbase`. `internal/domain` e `internal/app` seguem sem importar o framework (archtest verde).

### 0.8 Configuração: `internal/config`

A Fase 6 centraliza a configuração do binário único no pacote `internal/config` — fonte única para cifra, header do Pangolin, Google e Gemini. Substitui a leitura de env dispersa em `wiring.go`/`httpx` por um loader tipado, preservando os seams e o fail-closed da cifra.

- **Pacote de borda puro.** `internal/config` usa apenas a stdlib + `gopkg.in/yaml.v3`. **Não** importa o PocketBase nem `internal/platform/pb` — garantido por `TestConfigNaoImportaPocketBase` no archtest. Quem liga a config ao framework é `wiring.go` (e, futuramente, `cmd/psicoman`).
- **Struct tipada.** `Config{ DataDir, Log{Level}, Crypto{KeyID, Keys}, Pangolin{IdentityHeader}, Google{ClientID, ClientSecret, CalendarID, DriveFolder}, Gemini{APIKey} }`, espelhando `config.dev.yaml`. O YAML aninhado `llm.gemini.api_key` mapeia para `Gemini.APIKey` via um struct intermediário só de decode. `Config.Gemini.Enabled()` reporta o opt-in da IA pela **presença** da api_key (R10.3).
- **Ordem de descoberta do arquivo.** (1) `--config`/`PSICOMAN_CONFIG`; (2) `./config.dev.yaml`; (3) `./config.yaml`. Ausência de arquivo **não** é fatal — cai para defaults + env; a cifra segue fail-closed no boot (abaixo).
- **Precedência env > arquivo, com origem logada.** Para cada chave, env setada vence o arquivo. Mapeamento: `crypto.key_id`←`PSICOMAN_CIPHER_KEY_ID`, `crypto.keys`←`PSICOMAN_CIPHER_KEYS` (`id:base64,…`), `pangolin.identity_header`←`PSICOMAN_PANGOLIN_IDENTITY_HEADER`, `llm.gemini.api_key`←`PSICOMAN_GEMINI_API_KEY`; `data_dir` é informativo (o flag `--dir` do PB vence); `google.*` sem env por ora. `config.Load(opts) (Config, []OriginEvent, error)` devolve a origem (`arquivo`|`env`|`default`) de **cada** chave; o `buildServices` loga `config carregada chave=<k> origem=<src>` — **nunca** o valor de segredo (`crypto.keys`/`gemini.api_key`/`google.client_secret`), só chave + origem (`psicoman-seguranca-lgpd`).
- **Seam preservado e fail-closed inalterado.** O `wiring.go` ganha `var configLoader = config.Load` (fonte única) e mantém `var cipherConfigLoader = loadCipherConfig` com a mesma assinatura `func() (cipherConfig, error)` (os testes em processo o reatribuem). `loadCipherConfig` deriva `cipherConfig` de `Config.Crypto` (map → `[]crypto.KeySpec`); cifra ausente/incoerente continua **abortando o boot** em `buildServices`.

### 0.9 Google Calendar síncrono + PDF de cobrança

O **Google Calendar** é integrado de forma síncrona (único Google no caminho de request) e a **cobrança em PDF** é gerada e armazenada no GED. Gmail fica **fora de escopo** (D9b): o canal de envio é a terapeuta (WhatsApp), fora da plataforma.

- **Calendar atrás de interface + fake (`internal/integration/google`).** `CalendarClient{ FreeBusy, CreateEvent }` é a porta local; `restCalendarClient` fala com a Calendar API v3 sobre um `httpDoer` injetável (o `*http.Client` do OAuth2 carrega a credencial); `FakeCalendarClient` satisfaz a mesma interface nos testes — **nenhum** teste toca a rede real do Google. O `CalendarAdapter` adapta o cliente às portas `app.ConflictChecker` (freebusy) e `app.SessionCalendar` (evento). Escopo OAuth mínimo: `calendar`.
- **Freebusy antes de confirmar → 409.** Ao agendar (direto ou via confirmação de pedido), o `SessionService` consulta o freebusy do terapeuta; qualquer janela ocupada (inclui eventos externos) vira `app.ErrConflict` → HTTP 409 `conflito`, nunca sobrescreve. O evento criado inclui o paciente como convidado e os reminders configurados (default 1 dia + 30 min); o **Meet é criado apenas em sessão `online`** (presencial cria evento sem conferência).
- **Degrada sem credenciais, sem panic.** `wireCalendar` só ativa o Calendar real quando há `google.client_id` + `client_secret` + `calendar_id` **e** um refresh token (`PSICOMAN_GOOGLE_REFRESH_TOKEN`, seam provisório até a Fase 9). Faltando qualquer um, **não injeta nada**: a sessão agenda sem evento (no-op), com aviso logado no boot (sem vazar segredo). O fluxo OAuth 3-legged completo (persistência cifrada do refresh token) chega numa fase posterior.
- **PDF de cobrança no GED (file storage do PocketBase).** `internal/platform/pdf` gera um PDF 1.4 de uma página em **Go puro** (sem dependência externa). O `app.InvoiceService` compõe o documento a partir do débito + paciente, calcula o SHA-256, **deduplica por hash no escopo do paciente** (`GEDMetaRepository.FindByHash`) e, se novo, grava os bytes via `app.GEDBlobStore` e insere o metadado em `ged_file`, vinculando o arquivo ao débito (`SetPDF`). O `GEDBlobStore` é implementado em `platform/pb` sobre `app.NewFilesystem()` (local ou S3, conforme a config do PB) — o `rel_path` do domínio é o `fileKey` do bucket. É o **único** ponto que toca o storage de binários do GED; o domínio/app não importam o PocketBase.
- **Rotas (ator app).** `POST /v1/app/debts/{id}/invoice` gera/reaproveita o PDF e devolve o metadado (`InvoiceDTO`); `GET /v1/app/debts/{id}/invoice.pdf` baixa os bytes (`application/pdf`, fora do envelope JSON). A auditoria `debito_gerar` registra só `ged_file_id` (minimização — sem valor nem clínico).

### 0.10 Google Drive como fallback/DR via hook de backup

O sistema espelha os backups do PocketBase para o Google Drive como **fallback/DR**, disparado por um **hook no ciclo de backup** — fora do caminho de request. O Drive **nunca** é fonte da verdade nem é consultado por nenhuma operação de negócio (D9).

- **Cifra de BLOB dedicada (`crypto.BlobSealer`).** Distinta da cifra de campo (`domain.Cipher`, envelope texto base64 para uma nota clínica curta): o snapshot é um zip binário grande, então `BlobSealer.SealBytes/OpenBytes` cifra **bytes** com o mesmo primitivo (AES-256-GCM) e as **mesmas chaves** (`domain.KeyProvider`, sem chave nem config nova), num envelope **binário** compacto `magic "PSB1" | keyIDLen | keyID | nonce | ciphertext`. O `keyID` embutido habilita rotação na restauração; o header entra como AAD (autenticado). Round-trip, não-vazamento, rotação e falha com chave errada/header adulterado são cobertos por teste.
- **`internal/drivesync` + hook `OnBackupCreate`.** `Syncer.Register(app)` liga `app.OnBackupCreate().BindFunc`: o hook deixa o backup local acontecer (`e.Next()`), depois lê o snapshot do backups-filesystem (`app.NewBackupsFilesystem().GetReader(name)`), cifra com o `BlobSealer` e faz upload ao Drive. Interfaces locais `SnapshotUploader` e `BlobSealer` tornam o hook testável com fakes, sem acoplar ao `integration/google` nem tocar a rede.
- **Drive client (interface + fake + REST `drive.file`).** `google.DriveClient{ Upload }`; `restDriveClient` faz upload **multipart** (metadata JSON + bytes) à Drive API v3 sobre o mesmo `httpDoer` do Calendar; `FakeDriveClient` nos testes. Escopo OAuth mínimo: **`drive.file`** (só arquivos criados pela app, nunca o Drive pessoal inteiro). `driveUploaderAdapter` (wiring) adapta o `DriveClient` à porta `SnapshotUploader`, enviando o snapshot cifrado para a pasta fria (`google.drive_folder`).
- **Best-effort, degrada sem credenciais.** `wireDriveSync` registra o `Syncer` sempre; ativa o uploader real só com `client_id` + `client_secret` + `drive_folder` **e** refresh token. Faltando qualquer um, o `Syncer` roda com uploader nil: o hook é **no-op observável** (aviso logado), o backup local segue, sem 500. Mesmo com Drive configurado, falha de cifra/upload é **logada mas não propagada** — não invalida o backup local nem derruba o processo.
- **Rota de backup real.** `POST /v1/admin/backup` deixa de ser stub: chama `app.CreateBackup(ctx, "")` (timeout dedicado de 2 min), disparando o hook de DR. E2E prova que, sem Google, responde 202 sem 500.

### 0.11 IA Gemini (resumo revisável) + índice invertido de tags

A assistência por IA (Gemini) oferece resumo clínico + tagueamento, com **disparo explícito** pelo terapeuta, resultado **revisável** antes de consolidar, **opt-in** pela presença da chave, auditoria minimizada e um índice invertido de tags por paciente/ano (R10).

- **Gemini atrás de interface + fake (`internal/integration/gemini`).** `Summarizer{ Summarize(ctx, SummaryInput) (*SummaryResult, error) }`; `restSummarizer` chama o `generateContent` da API (pede `responseMimeType: application/json`, parseia `{summary, tags}`); `FakeSummarizer` nos testes — nenhum teste toca a API real. `SummaryInput` carrega **só o texto clínico** (minimização: sem id/nome de paciente). A `api_key` vem do `internal/config` (`llm.gemini.api_key`), **nunca** hard-coded nem logada.
- **Opt-in pela presença da chave (default OFF).** `wireDriveSync`/`buildAISummarizer` liga o Gemini real só quando `cfg.Gemini.Enabled()` (api_key não-vazia); sem chave, o `AISummarizer` injetado é nil e `AIService.Enabled()` é false. `POST /v1/app/sessions/{id}/ai-summary` com a IA desligada responde **422** (nunca 500). O opt-in é a presença da chave — não há flag on/off separada.
- **Ciclo revisável (`AIService`).** `Suggest` envia o texto, grava o resumo como **`sugerido`** (`ai_summary.resumo` é `SealedText`, cifrado na borda) e **devolve as tags sugeridas sem indexá-las** — o terapeuta revisa. `Consolidate(summaryID, reviewedTags)` marca o resumo como **`consolidado`** e só então grava as tags **revisadas** no índice. A auditoria `ai.process` registra apenas o fato (ator, `session_id`, contagem de tags) — **nunca** o texto enviado/retornado (minimização, psicoman-seguranca-lgpd).
- **Índice invertido de tags (`tag_index`).** Entrada `(patient_id, ano, tag) → session_ids`, tags em **claro** (indexáveis) enquanto o corpo clínico permanece `SealedText` cifrado. `TagIndexRepository.Upsert` faz merge idempotente; `FindByTag`/busca são **sempre isolados por paciente** (nenhuma consulta cruza pacientes, R10.2). `GET /v1/app/patients/{id}/tags?tag=&ano=` devolve as sessões do paciente que carregam a tag no ano.

### 0.12 Portal do paciente: auth real (substitui o SEAM)

O portal usa a **identidade real** da sessão do PocketBase (substitui o SEAM `X-Portal-Email` das primeiras fases) e materializa o cadastro/login do paciente (D2/D3).

- **Coleção de auth separada `patient_user` (Opção A).** `core.NewAuthCollection("patient_user")` (email + senha nativos, tokens, `auth-with-password`) com relation **UNIQUE** `patient_id` → `patient` (1:1). O `patient` segue intacto como a entidade de negócio (ULID, relations clínicas, `approval_status`, índices) — **sem** migração destrutiva. Migração `0040_patient_auth`.
- **Identidade real substitui o SEAM.** `httpx.PortalEmail` (header) vira `httpx.PortalAuthRecordID(e)`, que lê `e.Auth` — o auth record carregado pelo middleware global `loadAuthToken` do PocketBase a partir do `Authorization: Bearer <token>` — e confirma que a sessão é da coleção `patient_user`. O gate (`portal.RequireApproved`) e os handlers resolvem o paciente via a porta `app.PortalAuth.PatientIDByAuthID` (adapter `platform/pb` sobre `patient_user`). Só este ponto mudou — o resto do gate/handlers é o mesmo.
- **Login nativo do PocketBase.** O paciente obtém o token em `POST /api/collections/patient_user/auth-with-password` (endpoint nativo, liberado independentemente das rules nulas da coleção) e o envia aos `/v1/portal/*`. Não há reimplementação de auth.
- **Cadastro cria patient + credencial.** `POST /v1/portal/register` agora exige `password` (mín. 8); cria o `patient` (pendente) via `RegisterFromPortal` e, em seguida, a credencial `patient_user` vinculada (`PortalAuth.CreateLogin`, email verificado — a verificação por email está fora de escopo; o controle de acesso é o gate de aprovação).
- **Gate inalterado em semântica.** Pendente/rejeitado → 403 em recurso protegido; `register` e `me` ficam fora do gate (o paciente vê o próprio estado mesmo pendente); sem sessão → 401. Agora tudo sobre a identidade real. E2E cobre o caminho negado (pendente → 403) e o permitido (aprovado → 2xx) **pela mesma sessão**.
