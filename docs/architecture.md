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
- **Modo dev de identidade (`pangolin.dev_identity`, DEV ONLY).** Campo de config (só YAML, sem env) que relaxa o gate do Pangolin para rodar a API localmente — ex.: concluir o consentimento OAuth do Google pelo navegador (`GET /v1/admin/google/callback`), que sem o Pangolin receberia 401. Quando **não-vazio**, requisições a `/v1/app/*` e `/v1/admin/*` que chegam **sem** o header de identidade passam usando esse valor como ator; o **header real, quando presente, SEMPRE vence** (não é sobrescrito). Quando **vazio/ausente** (default), o comportamento é o de produção byte a byte (**401 sem header**), fail-closed por omissão — inclusive quando a leitura do config falha. `httpx.RequirePangolinIdentity(headerName, devIdentity)` recebe os dois valores resolvidos no boot; o `Register` lê o campo via `configLoader` e, quando ativo, loga um **WARN visível no boot** (`ATENÇÃO: modo dev de identidade ATIVO — gate do Pangolin relaxado, NÃO usar em produção`) sem vazar o valor da identidade. O default fail-closed mantém a matriz de autorização inalterada.
- **Admin UI nativa `/_/` e REST `/api/*` intactas.** Nenhum middleware do Pangolin incide sobre `/_/` (Dashboard com auth própria de superuser, segundo fator) nem sobre `/api/*` (REST automática já trancada por regra nil → **403** sem superuser, não 401). O middleware de header só cobre os grupos `/v1/app/*` e `/v1/admin/*`.
- **Gate de aprovação do portal (`/v1/portal/*`).** Middleware do grupo (`portal.RequireApproved`) resolve o paciente e **nega 403 `acesso_negado`** se `approval_status != "aprovado"`, EXCETO `POST /v1/portal/register` e `GET /v1/portal/me` (comparação por **path exato**, acessíveis a pendente/rejeitado para o paciente se cadastrar e ver o próprio estado). A identidade do paciente vem de um **seam** (`httpx.PortalEmail`, header `X-Portal-Email`); a auth real do paciente (sessão/`core.NewAuthCollection`) fica para a **Fase 9** — o seam é trocável sem mexer no gate.
- **Hook de reforço (defesa em profundidade).** Hook model-level (`OnRecordCreate`/`OnRecordUpdate`) em `internal/api/hooks.go` barra a escrita de `appointment_request` cujo paciente dono não esteja aprovado — por **qualquer** caminho, inclusive o adapter `platform/pb` (`app.Save`), que não passa pela chain HTTP. Escopo deliberadamente reduzido a `appointment_request` (única escrita originável pelo paciente no portal e com `patient_id` direto); `payment`/`ged_file`/`patient`/colunas clínicas ficam fora (ver `docs/decisions.md`). Invariante: agendar exige paciente aprovado, venha de quem vier.
- **Regra de dependência.** `httpx` (middleware de header, seams) importa só `core`+stdlib; o gate e o hook vivem em `internal/api`/`internal/api/portal`, que podem importar `app`/`service`/`pb`/`pocketbase`. `internal/domain` e `internal/app` seguem sem importar o framework (archtest verde).

### 0.8 Configuração: `internal/config`

A Fase 6 centraliza a configuração do binário único no pacote `internal/config` — fonte única para cifra, header do Pangolin, Google e Gemini. Substitui a leitura de env dispersa em `wiring.go`/`httpx` por um loader tipado, preservando os seams e o fail-closed da cifra.

- **Pacote de borda puro.** `internal/config` usa apenas a stdlib + `gopkg.in/yaml.v3`. **Não** importa o PocketBase nem `internal/platform/pb` — garantido por `TestConfigNaoImportaPocketBase` no archtest. Quem liga a config ao framework é `wiring.go` (e, futuramente, `cmd/psicoman`).
- **Struct tipada.** `Config{ DataDir, Log{Level}, Crypto{KeyID, Keys}, Pangolin{IdentityHeader, DevIdentity}, Google{ClientID, ClientSecret, CalendarID, DriveFolder}, Gemini{APIKey} }`, espelhando `config.dev.yaml`. `Pangolin.DevIdentity` (`pangolin.dev_identity`) é o bypass de DEV do gate (ver 0.7): só YAML, sem env, origem `arquivo|default`. O YAML aninhado `llm.gemini.api_key` mapeia para `Gemini.APIKey` via um struct intermediário só de decode. `Config.Gemini.Enabled()` reporta o opt-in da IA pela **presença** da api_key (R10.3).
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

### 0.13 Perfil do terapeuta no domínio rico (onda `pocketbase-finish`, R3)

Fecha o gap da `pocketbase-core`: o agregado `domain/therapist` existia sem porta/serviço/rotas, e o PDF de cobrança omitia o cabeçalho do terapeuta por isso. Agora o perfil é um **singleton lógico 1:1 por instância**, acessível pelo contrato `/v1`.

- **Porta + adapter + serviço.** `app.TherapistRepository` (`GetProfile`/`SaveProfile`/`ListLinks`/`CreateLink`/`DeleteLink`) implementada pelo adapter `platform/pb` sobre `therapist_profile`/`therapist_platform_link` (migração 0030). Campos JSON (`contacts` map, `location_ids` slice) via `rec.Set`/`rec.UnmarshalJSONField`; **nenhum campo é cifrado** — o perfil do terapeuta é dado operacional, não clínico de paciente. `app.TherapistService` orquestra o singleton: `Upsert` reusa o id do perfil existente e preserva `created_at` (idempotente 1:1 — gravar duas vezes atualiza, nunca duplica).
- **Rotas (ator app).** `GET /v1/app/therapist` (perfil + links), `PUT /v1/app/therapist` (cria/atualiza), `POST /v1/app/therapist/platform-links` e `DELETE /v1/app/therapist/platform-links/{id}`. Perfil sem nome → 422; link sem rótulo/URL → 422; adicionar link sem perfil → 404.
- **Portal só público (R3.4).** `GET /v1/portal/therapist` devolve **apenas** `TherapistProfile.PublicView()` (nome/foto/bio) — nunca CRP, email ou contatos. Fica **fora do gate** de aprovação (exceção de path exato em `portal.RequireApproved`): o paciente vê quem o atende antes mesmo de ser aprovado. E2E de não-vazamento confirma que CRP/email/telefone não aparecem no corpo.
- **Cabeçalho na cobrança (R3.3).** `InvoiceService.SetTherapist(repo)` (injetado no wiring) faz o `compose` imprimir **nome e CRP** no topo do PDF quando o perfil está configurado. É **best-effort**: sem perfil (`ErrNotFound`) ou sem a porta, o PDF sai sem cabeçalho, sem falhar a cobrança. E2E baixa o PDF e confere que os bytes contêm o nome/CRP.

### 0.14 OAuth 3-legged real do Google + token cifrado (onda `pocketbase-finish`, R2)

Substitui o seam de env (`PSICOMAN_GOOGLE_REFRESH_TOKEN`) pelo fluxo real de autorização, persistindo o refresh token cifrado em `google_token`. O env fica como **fallback só de dev** (⟨decisão R2⟩).

- **Domínio + porta + adapter.** `domain.GoogleToken` (refresh token como `SealedText`; scopes/expiry em claro). Porta `app.GoogleTokenRepository` (`Load`/`Save`, singleton lógico). Adapter `pb.NewGoogleTokenRepository` cifra o token na borda (`domain.SealFor`/`OpenInto`, como o prontuário) — o banco guarda só o envelope `v1.aes-256-gcm.…`, nunca o refresh token em claro (teste de não-vazamento confirma).
- **Serviço `GoogleAuthService`.** Orquestra a persistência (cifra pelo adapter), carrega o token no boot e mantém o **estado de reautorização** em memória. `SaveToken` reusa o id do singleton; rejeita refresh token vazio (o Google só o devolve com `access_type=offline` + `prompt=consent`). `Status(ctx)` → `ok|degraded` para a observabilidade (`/readyz`, Fase 3) e `/v1/admin/google/status`. Disciplina de dependência: o `app` NÃO importa `integration/google` — a troca code→token (que usa o SDK `oauth2`) fica no handler admin.
- **Fluxo de autorização (admin).** `google.AuthFlow` (em `integration/google`, único a tocar o `oauth2`): `GET /v1/admin/google/authorize` monta a URL de consentimento (`access_type=offline`, `prompt=consent`, escopos `calendar`+`drive.file`) e redireciona (302); `GET /v1/admin/google/callback` troca o `code` pelo refresh token e o persiste cifrado. Redirect URI derivado do request (`X-Forwarded-Proto/Host` atrás do Pangolin). Erro do Google → 422; troca falha → 502; sem credenciais do app → 422.
- **Boot: precedência do repo.** `resolveGoogleRefreshToken` prefere o token do `google_token` (cifrado) ao env `PSICOMAN_GOOGLE_REFRESH_TOKEN` (fallback de dev), logando a **origem** sem vazar o segredo. `wireCalendar`/`wireDriveSync` recebem o `GoogleAuthService`; sem token utilizável, sinalizam reautorização necessária (`FlagReauthorization`) e degradam sem panic (o Calendar agenda sem evento; o Drive DR não espelha), como antes.
- **Testes.** Unit do `GoogleAuthService` (save/status/reauth/precedência); adapter round-trip + não-vazamento do refresh token; E2E em processo do fluxo de callback com **servidor OAuth fake** (seam `google.SetAuthEndpointForTest`) — nenhum teste toca o Google real; status degraded→ok e não-vazamento do token nas respostas.

### 0.15 Observabilidade: readiness/liveness (onda `pocketbase-finish`, R5)

Fecha o gap de observabilidade: só havia `/v1/health`. Agora há `/v1/livez` (liveness) e `/v1/readyz` (readiness), fora da auth (probes de orquestrador), no envelope padrão.

- **`/v1/livez`** — o processo está vivo: 200 fixo, sem checar dependências. É o sinal de "reiniciar se não responder".
- **`/v1/readyz`** — pronto para receber tráfego, com três checks no corpo (`checks.cipher`, `checks.db`, `checks.google`): **(1) cifra montada** — round-trip `Seal→Open` de um texto fixo (`readyz-probe`), nunca dado real; **(2) DB acessível** — lookup de uma coleção conhecida (`patient`) exercita o SQLite; **(3) estado do OAuth do Google** — `GoogleAuthService.Status` (`ok|degraded`, da Fase 2). Cifra ou DB com erro → **503 `nao_pronto`**. O Google **degradado NÃO torna a instância não-pronta** (a app opera sem Google, só degrada Calendar/Drive) — é reportado no corpo para a operação agir (reautorizar).
- **Cifra exposta ao check.** `service.Services.Cipher` carrega a cifra montada no boot só para o round-trip do readiness; nenhum dado real é cifrado ali.
- **Métricas: adiadas (⟨decisão R5⟩).** O timing por rota já é logado (`httpx.Timing`/`Logging`), cobrindo a observabilidade mínima; um endpoint de métricas (Prometheus) é desejável mas foi conscientemente adiado para uma onda futura (R5.2 admite a decisão explícita de adiar).
- **Testes.** E2E (`/livez` 200; `/readyz` pronto com `google=degraded` sem credenciais) + in-process (livez; readyz degraded; readyz `google=ok` após o callback OAuth com servidor fake — prova que o `/readyz` reflete o estado real do OAuth; não-vazamento do refresh token no corpo).

### 0.16 Núcleo utilizável: locais, origens e planos (onda `pocketbase-finish`, R1 sub-fase 1)

Expõe no `/v1/app` os recursos que tinham repositório/porta mas nenhuma superfície HTTP, destravando o **débito por sessão ponta a ponta via API** (R1.7/R1.9). Mesma estratégia das demais fases: handler fino → serviço enxuto → DTO explícito.

- **Locais + disponibilidades (R1.7).** `LocationService` (CRUD `scheduling.Location` + janelas `Availability`). Rotas `GET/POST /v1/app/locations`, `GET/PATCH/DELETE /v1/app/locations/{id}`, `GET/POST /v1/app/locations/{id}/availability`, `DELETE /v1/app/locations/availability/{id}` (rota estática registrada antes de `/locations/{id}` por especificidade do `http.ServeMux`). Validação de modalidade/periodicidade/horário no agregado → 422.
- **Origens (R1.7).** `OriginService` (CRUD `patient.Origin`) — base do ROI por canal. `GET/POST /v1/app/origins`, `PATCH/DELETE /v1/app/origins/{id}`.
- **Planos por paciente (R1.9).** `PlanService` (CRUD `billing.Plan`). `GET/POST /v1/app/patients/{id}/plans`, `DELETE /v1/app/plans/{id}`. **Um plano por sessão vigente é o que habilita o débito automático**: o `BillingService.OnSessionFinished` (hook de finalização) já lê `GetActiveByPatient` e gera o débito com o valor do plano. Com a rota de plano, o fluxo fecha só pela API (sem seed via superuser).
- **Comprovante de pagamento (R1.9).** `PaymentService.Pay` aceita um comprovante opcional (`receipt_base64`+`receipt_mime` no corpo de `POST /v1/app/debts/{id}/pay`), gravado no GED (dedup por SHA-256 no escopo do paciente) vinculado ao pagamento e ao débito — mesma disciplina do PDF de cobrança. Sem o GED injetado, um comprovante é rejeitado com validação.
- **Jornada ponta a ponta (E2E headline).** Cadastrar paciente → criar plano por consulta → criar sessão → finalizar → **débito gerado automaticamente com o valor do plano** → cobrança em PDF → pagar com comprovante → débito `pago`. Prova que o débito por sessão funciona só pela API.
- **Testes.** Unit dos 3 serviços (create/validação; disponibilidade; plano fechado sem valor → 422) + comprovante (grava no GED vinculado; sem GED → validação); E2E (contrato locais/origens; jornada completa; 422 de plano inválido).

### 0.17 Telas iniciais: feed, história, métricas, calendário (onda `pocketbase-finish`, R1 sub-fase 2)

Expõe no `/v1/app` as features de produto cujos serviços já existiam no container (`DayFeedService`, `HistoryService`, `SessionMetricsService`) mas estavam sem rota. Puro trabalho de superfície HTTP (handler fino + contrato), sem regra nova.

- **Feed do dia (R1.1).** `GET /v1/app/day-feed?window=hoje|7d|recentes`. Resposta = `clinical.DayFeed` (resumo + eventos ordenados, transversal a pacientes). Nunca traz conteúdo clínico; a leitura é auditada (`AuditActionRead`) pelo serviço.
- **História do paciente (R1.2).** `GET /v1/app/patients/{id}/history`. Resposta = `[]clinical.HistoryItem`, composta no servidor (cadastro, sessões, remarcações do audit, notas, anamnese, débitos, pagamentos, arquivos). Os itens carregam só **rótulos legíveis** (`Titulo`/`Resumo`), nunca o `SealedText` das notas/anamnese — leak-safe por construção do domínio. Leitura auditada.
- **Métricas de sessão (R1.3).** `GET /v1/app/patients/{id}/session-metrics?de=&ate=`. Resposta = `clinical.PatientMetrics` (comparecimento, cancelamento, quebra por categoria, faturado). Recorte opcional por período.
- **Calendário (R1.4).** `GET /v1/app/sessions?patient_id=&from=&to=`. Com `patient_id`, lista as sessões do paciente (recorte opcional por `[from,to)`); sem ele, exige `from`+`to` e lista por intervalo (`SessionService.ListByRange`) — sem `patient_id` nem intervalo → 422 (guarda contra varredura total).
- **Serialização direta do view-model.** Os tipos `clinical.DayFeed`/`HistoryItem`/`PatientMetrics` já têm tags JSON e são leak-safe (sem `SealedText`); são serializados direto no envelope (view-model do domínio), enquanto o calendário usa o `SessionDTO` do ator app.
- **Testes.** Os serviços já tinham unit; a Fase adiciona E2E de contrato + **não-vazamento clínico**: cria uma nota com conteúdo secreto e confirma que ele não aparece no feed, na história nem no calendário; valida `taxa_comparecimento` nas métricas e o 422 do calendário sem filtro.

### 0.18 Docs operacionais + infra do binário único (onda `pocketbase-finish`, R6)

Reescreve a documentação e a infra de container para a arquitetura vigente (processo único), removendo o modelo antigo de dois binários (`cmd/admin`/`cmd/portal`, `dev_mode`, portas 8080/8081, Gmail).

- **`docs/deploy.md`** reescrito: compilar `bin/psicoman`; `config.yaml` (cifra fail-closed, env sobrescreve); Google OAuth com os escopos mínimos `calendar`+`drive.file` (sem Gmail) e a autorização real via `GET /v1/admin/google/authorize` → callback (Fase 2); Pangolin na borda com upstream `127.0.0.1:8090` e identidade por prefixo; **um** systemd unit (`serve --http --dir --config`, migrations no boot, admin UI em `/_/`); atualização manual com backup + rebuild + validação de `/readyz`. A nota de "desatualizado" foi removida.
- **`docs/stack-local.md`** reescrito para `docker compose up --build` de **um** serviço; probe por `/v1/livez`; `/v1/readyz` reporta `google: degraded` em teste (esperado, não bloqueia prontidão); alternativa `make serve` sem Docker.
- **Infra consertada** (coerência com os docs): `Dockerfile` agora compila `./cmd/psicoman` (Go 1.27, estático) com `ENTRYPOINT psicoman` e `serve --http 0.0.0.0:8090 --dir /data`; `docker-compose.yml` vira um único serviço com a chave de cifra de teste por env (`PSICOMAN_CIPHER_*`); `deploy/config.docker.yaml` reescrito no formato atual. A imagem foi validada: build OK, container sobe, `/v1/livez` e `/v1/readyz` respondem 200.
- **Scripts legados removidos** (auditoria): `scripts/deploy.sh`, `update-server.sh`, `stack-up.sh`, `stack-down.sh`, `seed.sh` eram do modelo de dois binários e estavam órfãos (o Makefile não tinha mais alvos `stack-*`). Foram apagados para não induzir um operador a rodar tooling quebrada; a operação segue os passos manuais/`docker compose` dos docs.

### 0.19 Custos, ROI e relatório por local (onda `pocketbase-finish`, R1 sub-fase 3)

Expõe o que `CostService`, `ReportService` e `SessionMetricsService` já faziam. Handlers finos em `api/app/costs.go`; dinheiro sempre em centavos (`*_cents`).

- **CRUD de custos.** `GET/POST /v1/app/cost-categories` (kind `local|crp|infra|plataforma`), `GET/POST /v1/app/cost-items`, `DELETE /v1/app/cost-items/{id}`. Um item com `origin_id` (custo de plataforma) alimenta o ROI por canal.
- **Custo de sessão.** `GET /v1/app/sessions/{id}/cost` devolve o custo atribuído ao finalizar com `consider_cost` (direto para `por_sessao`, rateio nos demais); 404 se não houver.
- **Relatório de custos.** `GET /v1/app/reports/costs?de=&ate=` → `by_kind_cents` e `by_patient_cents` (período inválido 400; invertido 422). O ROI (`/reports/roi`) já existia.
- **Relatório de sessões por local (R1.5).** `GET /v1/app/locations/{id}/session-report?de=&ate=`; sem período, usa o **mês corrente** (America/Sao_Paulo). Agrega por categoria e soma o faturado das realizadas.
- **Testes.** E2E de jornada: categoria → item ligado à origem → local por sessão → sessão finalizada com custo → custo da sessão (5000, direto), relatório de custos, ROI contendo a origem, session-report (1 realizada, faturado 18000), e os caminhos negativos (422/400/404).

### 0.20 Pedidos de agendamento pelo portal (onda `pocketbase-finish`, R1 sub-fase 4)

Fecha o fluxo "paciente pede → terapeuta confirma", usando a coleção `appointment_request` e o hook de aprovação que já existiam.

- **Serviço.** `AppointmentService`: `Request` (exige paciente **aprovado** e, se informado, local existente; nasce `pendente`), `ListPending`, `ListByPatient`, `Confirm` e `Refuse`. Só pedidos `pendente` mudam de estado (senão 409).
- **Confirmar = criar a sessão.** `Confirm` chama `SessionService.Create` com status `agendada` e `RequestID`: reaproveita a checagem de conflito (freebusy) e a criação do evento de Calendar. Conflito → 409 e o pedido **segue pendente** (nenhuma sessão criada). A modalidade vem do local (ou `online` sem local).
- **Rotas.** Portal (dentro do gate de aprovação; identidade da sessão, nunca do corpo): `POST/GET /v1/portal/appointment-requests`, com isolamento por paciente. Terapeuta: `GET /v1/app/appointment-requests` (fila), `POST .../{id}/confirm` (201 + sessão) e `POST .../{id}/refuse`.
- **Defesa em profundidade (3 camadas).** Gate do portal (403 para não-aprovado) → regra no serviço (`Request` recusa não-aprovado) → hook de modelo em `appointment_request`. A nota livre do paciente é limitada a 500 caracteres.
- **Testes.** Unit (aprovado/pendente/local inexistente/slot inválido; confirmar cria sessão `agendada` com `RequestID`; conflito mantém pendente; reconfirmar/recusar 409). E2E: pendente 403, sem sessão 401, validações 400/422, isolamento A×B, fila do terapeuta, confirmar→sessão visível só para A, recusar, 404/409 e app sem header 401.

### 0.21 Segregação de exposição na borda (nginx + Pangolin) — Opção A

Decisão do Rafael (pós-auditoria): separar a **exposição** das três superfícies sem reabrir o invariante "um processo, um listener, três prefixos". A separação é 100% na **borda**, o binário Go fica intocado.

- **Duas bordas, um binário.** O `cmd/psicoman` segue escutando só em `127.0.0.1:8090` com os grupos `/v1/{app,portal,admin}`. Quem separa a exposição é a infra à frente:
  - **nginx público** (`deploy/nginx/portal.conf`): única superfície na internet. Encaminha SOMENTE `/v1/portal/*` + `/v1/livez|readyz|health`; **nega com 404** `/v1/app`, `/v1/admin`, `/api/`, `/_/`, `/v1/swagger`. Endurecimento L7: rate limit por IP (30 r/min, burst 20 → 429), `client_max_body_size 2m`, timeouts curtos, `limit_conn`, limpeza do header `X-Pangolin-User` recebido de fora (anti-spoofing) e cabeçalhos de segurança. Não termina TLS no compose (o Pangolin/LB acima faz).
  - **Pangolin**: `/v1/app/*` e `/v1/admin/*`, injetando o header de identidade. Nunca passa pelo nginx público.
- **Compose espelha a produção.** `docker-compose.yml`: o serviço `psicoman` não publica porta no host (`expose: 8090`, interno); o `nginx-portal` (nginx:1.27-alpine) publica a 8080 e monta o `.conf`. `docker compose up` sobe a topologia real; a admin UI `/_/` e o app só são alcançáveis internamente (em dev, publica-se a 8090 do psicoman temporariamente — ver `stack-local.md`).
- **Defesa em profundidade, não substituição.** O gate de aprovação, a cifra e as validações continuam no app; o nginx é a camada extra que segrega a exposição e absorve abuso de rede antes de chegar ao binário.
- **Validação (manual, reproduzível):** com a stack no ar, portal/saúde → 200/401/422; app/admin/api/`_/`/swagger → 404 na borda; header de Pangolin forjado ignorado (404); 60 POSTs rápidos → mistura de 2xx/422 e 429 (rate limit ativo).
