# auditoria-profunda — Design

Como implementar os requisitos de [`requirements.md`](./requirements.md), respeitando os invariantes herdados (binário único; `domain`/`app`/`config` não importam o PocketBase nem a lib de métricas; cifra na borda + DTO/`Reveal`; contrato `/v1` por ator com envelope PT-BR; segredos nunca em log/repo). Nada aqui reabre a arquitetura-base nem as decisões já tomadas (R1 a R8 do requirements e o registro de decisões do Rafael de 2026-11-05).

> **Convenções deste documento.** **Recomendação de design** marca uma proposta minha onde o requirements deixou a escolha para o design. **SUPOSIÇÃO NÃO VERIFICADA** marca afirmação sobre código ou serviço externo que não confirmei lendo ou rodando; cada uma tem uma tarefa de verificação (spike) antes de depender dela. **VERIFICADO (2026-11-05)** marca o que foi resolvido por leitura de código ou execução local nessa data, com a evidência ao lado; as tarefas de spike correspondentes continuam no plano só para **reconfirmar rapidamente** (o ambiente pode mudar). O que sobrou com o rótulo de suposição é **NÃO VERIFICÁVEL LOCALMENTE** (exige credencial real do Google) e tem spike manual descrito. O relatório completo está em `.agents/tasks/auditoria-profunda-spec/resolver-suposicoes.md` (git-ignored). As imprecisões factuais que a verificação deste design encontrou no requirements (por exemplo, o `nil` de `NewProntuarioService` ser o relógio e não a auditoria, ou `appointment-requests` ser autenticada) já foram corrigidas no próprio `requirements.md`, que está alinhado com este documento.

**Stack travada (aprovada junto com este design):** Go 1.27 (`go.mod`: `go 1.27`, toolchain local `go1.27.1`), PocketBase `v0.40.6` embarcado, SQLite, `github.com/prometheus/client_golang` **`v1.25.0`** (nova; última estável compatível com `go 1.27` na data da investigação), `golangci-lint` **`v2.15.0`** (ferramenta de dev, não entra no `go.mod`). Nenhuma outra dependência de módulo nova (rate limit e store de `state` em stdlib).

---

## 0. Visão geral e mapa de mudanças

| Req | Decisão em uma linha | Núcleo do código |
|-----|----------------------|------------------|
| R1 | `ProntuarioService` ganha `SetAudit`; leitura/escrita do prontuário gravam `audit_log` sem clínico | `internal/app/prontuario_service.go`, `internal/api/wiring.go` |
| R2 | Snapshot inteiro oficializado (ADR + steering); retenção por contagem (N=14) feita pelo `drivesync` | `docs/decisions.md`, steering, `internal/drivesync`, `DriveClient.List/Trash` |
| R3 | Pasta de DR criada via `files.create`; id persistido em coleção própria `drive_dr`; reuso por config > persistido > busca > criação | `internal/integration/google/drive.go`, `internal/drivesync/folder.go`, migration `0050` |
| R4 | `golangci-lint` v2.15.0 instalado via `go install`, `.golangci.yml` migrado, `make lint` falha de verdade, passo no CI, fase de triagem | `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml` |
| R5 | Unit no service (fake de `AuditRepository`) + E2E real lendo `GET /v1/admin/audit-log` | `internal/app/*_test.go`, `test/e2e/` |
| R6 | Steerings sem Gmail; `architecture.md`/`deploy.md` com Drive DR real; tabela da Fase 10 consolidada | `.kiro/steering/*`, `docs/*`, `.kiro/specs/pocketbase-finish/tasks.md` |
| R7.1 | Limitador em memória (janela fixa por chave) no app + nginx por IP; chave por hash do e-mail (register) e do `patient_id` (agendamento) | `internal/api/httpx/ratelimit.go` |
| R7.2 | `state` OAuth: store em memória (TTL 10 min, uso único) **mais** cookie HttpOnly que amarra ao navegador | `internal/app/oauth_state.go`, `internal/api/admin/google_oauth.go` |
| R7.3 | Só checklist operacional | `docs/deploy.md`, `docs/release-notes.md` |
| R7.4 | Ator propagado por `context` (`domain.WithActor`), middleware após `RequirePangolinIdentity`; `AuditService.Record` resolve o ator | `internal/domain/actor.go`, `internal/api/httpx`, `internal/app/audit_service.go` |
| R8 | Listener dedicado (só se `metrics.addr` configurado), registry próprio, porta de negócio em `internal/app`, middleware raiz do PocketBase | `internal/platform/metrics`, `internal/api/httpx/metrics.go`, `internal/api/metricsserver.go`, `internal/app/metrics.go` |

R9 e R10 são rastreabilidade e DoD, tratados na [matriz de cobertura](#12-matriz-de-cobertura-requisito--seção) e no [fechamento](#11-fechamento-de-fase-e-dod).

### Fatos verificados que sustentam o design

- `wiring.go:145`: `app.NewProntuarioService(prontuarioRepo, nil)`; o `nil` é o `clock.Clock`. Audit é injetado por `SetAudit` em `SessionService` (padrão a seguir). `AuditService.Record(ctx, actor, action, entity, entityID, meta)`; 11 chamadas em `internal/app` usam o ator literal `"sistema"`.
- `GET /v1/admin/audit-log?limit=` já existe (`internal/api/admin/ops.go`) e devolve `actor`, `action`, `entity`, `entity_id`, `meta`. **Isso permite E2E real da trilha de auditoria sem endpoint novo.**
- PocketBase `v0.40.6`: `BuildMux()` registra um catch-all `/` quando não há rota `/` (`tools/router/router.go`), que devolve `NewNotFoundError`. Os middlewares do router raiz (`e.Router.BindFunc`) rodam também nesse catch-all. O `http.ServeMux` preenche `Request.Pattern` com o padrão casado (ex.: `GET /v1/app/patients/{id}`; no catch-all, `/`). O `ResponseWriter` do PB rastreia `Status()`/`Written()`, mas **não conta bytes**. Prioridades dos middlewares padrão do PB: de `-1040` (activity logger) a `-1000` (rate limit).
- `core.App.CountRecords(collection, exprs...)` existe (`core/record_query.go`), útil para os gauges de fila.
- Há `.github/workflows/ci.yml` (`make check` e `go test -race ./internal/...`), sem golangci-lint.
- `Services` já tem um campo `Metrics *app.SessionMetricsService` (métricas de sessão do produto). O campo da porta de observabilidade se chama **`Obs`**, para não colidir.

---

## 1. R1 — Auditoria LGPD no caminho clínico

**Decisão.** Auditar no **service** (não no handler): o handler continua fino e o comportamento fica coberto por unit com fakes. `ProntuarioService` ganha o campo `audit *AuditService` e o método `SetAudit(a *AuditService)` (mesmo padrão de `SessionService.SetAudit`; não quebra o construtor nem os testes existentes). O ator **não** vira parâmetro: vem do `context` (ver [R7.4](#6-r74--ator-do-audit-pela-identidade-real)), de modo que as assinaturas `GetChart`, `AddNote`, `UpsertAnamnesis` não mudam.

**Arquivos.** `internal/app/prontuario_service.go`, `internal/api/wiring.go` (após `app.NewProntuarioService(prontuarioRepo, nil)` adicionar `prontuario.SetAudit(audit)`), `internal/domain/audit.go` (sem ação nova: reusa `AuditActionRead`, `AuditActionCreate`, `AuditActionUpdate`).

**Pontos de auditoria** (sempre best-effort, no padrão do projeto `if s.audit != nil { _ = s.audit.Record(...) }`, e **somente após sucesso** da operação):

| Operação | Ação | `entity` / `entity_id` | `meta` (nunca o texto) |
|----------|------|------------------------|------------------------|
| `GetChart` | `AuditActionRead` | `prontuario` / `patientID` | `{patient_id, notas: len(notes), anamnese: bool}` |
| `AddNote` | `AuditActionCreate` | `note` / `note.ID` | `{patient_id, session_id}` |
| `UpsertAnamnesis` (novo, ramo `ErrNotFound`) | `AuditActionCreate` | `anamnesis` / `a.ID` | `{patient_id}` |
| `UpsertAnamnesis` (existente) | `AuditActionUpdate` | `anamnesis` / `a.ID` | `{patient_id}` |

O handler `GetPatientChart` chama `Patients.Get` antes do `GetChart`; só `GetChart` audita a leitura clínica (a leitura do cadastro já existente não é clínica e fica como está).

**Falha de auditoria.** O requirements pede trilha, mas o `AuditService` documenta best-effort e não derrubar a operação. **DECIDIDO:** manter best-effort (um prontuário ilegível por falha do `audit_log` seria pior para a terapeuta do que uma lacuna pontual na trilha), **mas tornar a falha observável**: `AuditService.Record` passa a (a) logar com `slog.Error("falha ao gravar audit_log", "acao", action, "entidade", entity)` (sem `meta`, sem ids) e (b) incrementar `AuditFailed(action)` na porta de métricas (R8.7). **Injeção (verificado: hoje `AuditService` só tem `repo` e `clock`, e `internal/app` não importa `log/slog`):** `AuditService` ganha dois campos opcionais, `log *slog.Logger` (default `slog.Default()`; `log/slog` é stdlib e permitido em `app`) e `metrics Metrics` (default `NopMetrics{}`), com `SetLogger(*slog.Logger)` e `SetMetrics(Metrics)`. O `buildServices` chama `audit.SetLogger(a.Logger())` (o `*slog.Logger` do PocketBase, para a falha cair no mesmo destino dos demais logs) e `audit.SetMetrics(obs)`. O log fica dentro de `Record` (os chamadores usam `_ =`), então nenhum ponto de chamada muda. Registrar a escolha no ADR de R1. Alternativa descartada: falhar a leitura quando o audit falha (fail-closed): ganha rastreabilidade estrita, mas cria indisponibilidade clínica por defeito de uma tabela auxiliar.

**Erros.** Nenhum novo contrato HTTP. Erro de `Record` nunca chega ao chamador.

**Testes (R5).**
- Unit em `internal/app/prontuario_service_test.go` com fake de `AuditRepository` (**VERIFICADO:** o fake é `fakeAuditRepo` em `internal/app/fakes_test.go` (pacote `app`, mesmo pacote do teste novo; não está em `feed_service_test.go`), com `Insert`, `List`, `ListByEntity` e o helper `actionsFor(entity, entityID)`; `newAudit()` devolve `(*AuditService, *fakeAuditRepo)` pronto para `prontuario.SetAudit(audit)`. O repositório de prontuário fake é `fakeProntuarioRepo` no mesmo arquivo. Evidência: `grep -n fakeAuditRepo internal/app/fakes_test.go`): `GetChart`, `AddNote`, `UpsertAnamnesis` (criar e atualizar) gravam a ação certa; `meta` não contém o claro (varre valores `string` e compara com o texto enviado); sem sucesso, nada é gravado; `audit == nil` não panica.
- E2E em `test/e2e/audit_prontuario_test.go`: sobe o binário (`startServer`), cria paciente, `POST /patients/{id}/notes`, `PUT /patients/{id}/anamnesis`, `GET /patients/{id}`, todos com `X-Pangolin-User`; lê `GET /v1/admin/audit-log` e confere ações/entidades, o `actor` igual ao header e a ausência do texto clínico no JSON inteiro da resposta.

**Docs.** `docs/architecture.md` (seção de auditoria: lista de operações auditadas), `docs/decisions.md` (ADR "Auditoria no caminho clínico"), `docs/release-notes.md`.

---

## 2. R2 — Snapshot inteiro oficializado e retenção

**Decisão (já tomada).** Nenhuma mudança no envio: cada backup sobe o snapshot inteiro cifrado (AES-256-GCM) como `<backup>.enc`. O arquivo do PocketBase já é um zip; o código não recompacta, e o steering passa a dizer isso com precisão.

**Trabalho documental.**
1. ADR em `docs/decisions.md` (topo, formato data/contexto/decisão/consequências): "Backup ao Drive é snapshot inteiro". Consequências: crescimento linear do acervo, custo de banda proporcional ao banco, simplicidade e restauração em um arquivo. Referenciar o ADR de retenção (abaixo).
2. Steering `psicoman-google-api.md` (seção Drive): trocar a linha "Backup incremental por hash (manifesto no Drive)..." por "Snapshot inteiro por execução, cifrado (AES-256-GCM) antes do upload; o zip do PocketBase já é compactado. Retenção por contagem feita pelo `drivesync`." Edição do steering é tarefa de implementação (não nesta etapa de spec).

**Retenção (R2.4) — DECIDIDO (ver "Decisões fechadas"): manter os N snapshots mais recentes, N=14 por padrão, limpeza feita pelo próprio `drivesync` logo após um upload bem-sucedido.** Justificativa: o backup é diário em produção (a verificar em `docs/deploy.md`), 14 cobre duas semanas de pontos de restauração e limita o crescimento; fazer no mesmo hook evita um cron adicional; a contagem (não a idade) protege contra o caso "backup parou e a limpeza apaga tudo". A alternativa "retenção manual" foi descartada porque o acervo cresce sem aviso e o requirement R8.6 já pede a métrica de snapshots mantidos.

Config: `google.drive_retention` (int, **default 14**, `0` = nunca apagar, retenção manual). Em `internal/config`, `yamlDoc.Google.DriveRetention *int` (ponteiro distingue "ausente" de `0`); `GoogleConfig.DriveRetention int`; origem registrada com `resolve`-like (arquivo|default). Sem env (consistente com `google.*`). Valor negativo é inválido: `Load` devolve erro de config (`google.drive_retention deve ser >= 0`), o boot aborta com mensagem clara.

**Interface nova (em `internal/integration/google/drive.go`).**

```go
// DriveFile é a visão mínima de um arquivo no Drive.
type DriveFile struct {
    ID          string
    Name        string
    CreatedTime time.Time
}

// DriveListInput filtra arquivos criados pelo app dentro de uma pasta.
type DriveListInput struct {
    FolderID string
    AppProps map[string]string // ex.: {"psicoman_kind":"snapshot"}
}

type DriveClient interface {
    Upload(ctx context.Context, in DriveUploadInput) (*DriveUploadResult, error)
    // List devolve os arquivos não-lixeira da pasta que casam AppProps, mais recentes primeiro.
    List(ctx context.Context, in DriveListInput) ([]DriveFile, error)
    // Trash move o arquivo para a lixeira (reversível por 30 dias).
    Trash(ctx context.Context, fileID string) error
    // FindFolder / CreateFolder: ver R3.
    FindFolder(ctx context.Context, name string, appProps map[string]string) (id string, found bool, err error)
    CreateFolder(ctx context.Context, name string, appProps map[string]string) (id string, err error)
    // GetFolder (files.get?fields=id,trashed): revalida o id persistido; ver R3.
    GetFolder(ctx context.Context, id string) (trashed bool, err error)
}
```

`DriveUploadInput` ganha `AppProps map[string]string` (gravado como `appProperties` no metadata do multipart). Os snapshots passam a ser gravados com `{"psicoman_kind":"snapshot"}`.

**Mecânica da limpeza** (`internal/drivesync/retention.go`; o `Syncer` obtém `List`/`Trash` pela interface `SnapshotStore` definida em [§3.1](#31-contrato-interno-do-drivesync); função pura testável `selectToTrash(files []File, keep int, justUploadedID string) []string`):
1. Só roda se o upload do snapshot atual retornou sucesso e `retention > 0`.
2. `List` (REST: `files.list` com `q='<folder>' in parents and trashed=false and appProperties has {key='psicoman_kind' and value='snapshot'}`, `orderBy=createdTime desc`, paginação por `pageToken` com teto de 10 páginas de 100).
3. Mantém os `N` mais recentes **garantindo que o arquivo recém-enviado esteja no conjunto mantido** (consistência eventual da listagem); o resto vai para `Trash`.
4. Trava de segurança em duas camadas: (a) o **`List` do adapter** (wiring) só devolve arquivos com `appProperties psicoman_kind=snapshot` cuja pasta pai é a pasta de DR (a query `q` já filtra por `in parents` e `appProperties`; `drivesync.File` não carrega pasta pai nem props); (b) `selectToTrash` aplica `strings.HasSuffix(Name, ".enc")` sobre o que recebeu e ignora o resto. Arquivos antigos sem `appProperties` (anteriores a esta onda) **não** são tocados (limpeza manual, documentada).
5. Falha em `List`/`Trash` é logada em `Warn` com contador `psicoman_drive_retencao_falhas_total`, nunca propaga e nunca invalida o upload.

Usa **lixeira** (`Trash`, `PATCH files/{id}` com `{"trashed":true}`) e não `delete`: um erro de lógica é recuperável por 30 dias. Custo: a lixeira ocupa cota até esvaziar. **VERIFICADO PELA DOCUMENTAÇÃO OFICIAL (2026-11-05):** o escopo `drive.file` "funciona com todos os recursos REST da Drive API" e dá acesso por arquivo, a arquivos criados ou abertos pelo app (`developers.google.com/workspace/drive/api/guides/api-specific-auth`); logo `files.list`, `files.update` (`trashed`) e `files.create` de pasta são permitidos para o que o próprio app criou. Falta só a **confirmação em runtime**, coberta pelo spike manual da tarefa 4.1 com credencial de dev (fora da suíte automatizada). Se, contra a documentação, a listagem não for permitida, o `drivesync` desativa a retenção com `Warn` único e a métrica de snapshots fica ausente; o ADR registra "retenção manual" como fallback.

**Impacto em testes.** `FakeDriveClient` ganha `Files []DriveFile`, `ListErr`, `TrashErr`, `Trashed []string`, `FolderCalls`, etc. (ver R3). Unit de `selectToTrash` (tabela: menos que N, exatamente N, mais que N, recém-enviado fora da listagem, arquivos sem prop). `drivesync_test.go`: upload ok + retenção apaga os excedentes; `Trash` com erro não falha o backup; `retention=0` não lista.

**Docs.** ADR (snapshot inteiro + retenção), `docs/deploy.md` (chave `google.drive_retention`, lixeira), `docs/architecture.md §0.10`, `config.example.yaml`.

---

## 3. R3 — Pasta de DR criada automaticamente

**Decisão.** A pasta é resolvida de forma **lazy** no primeiro backup (não no boot, para não depender de rede para subir). Ordem de resolução, parando no primeiro sucesso:

1. `google.drive_folder` do config, se informado: usado como está. **Nunca** cria outra pasta e **nunca** persiste (o operador manda).
2. Id persistido na coleção `drive_dr`.
3. Busca no Drive por pasta criada pelo app: `files.list` com `q=mimeType='application/vnd.google-apps.folder' and trashed=false and name='Psicoman DR' and appProperties has {key='psicoman_role' and value='dr_folder'}`. Cobre a perda do estado local (instância nova restaurada de um snapshot sem o registro) e evita pasta duplicada. Persiste o id achado.
4. Criação via `files.create` (`POST https://www.googleapis.com/drive/v3/files`, JSON `{"name":"Psicoman DR","mimeType":"application/vnd.google-apps.folder","appProperties":{"psicoman_role":"dr_folder"}}`). Persiste o id.

**Observação empírica sobre `drive.file` e pais criados fora do app (2026-11-05, conta real do Rafael).** Um upload com `parents:[<id de pasta criada à mão na interface do Drive>]` **funcionou** com o escopo `drive.file` (o backup foi espelhado): o escopo aceita, ao menos para **upload**, um pai criado fora do app. Já a **busca** (passo 3: `files.list` por nome/`appProperties`) só enxerga o que o app criou ou abriu. Consequência de projeto: a pasta criada à mão do Rafael **não será encontrada** pelo passo de busca, e isso é o comportamento correto: se `google.drive_folder` estiver configurado, ele é usado (passo 1, já previsto); sem config, busca e, não achando, cria "Psicoman DR". Quem quiser manter a pasta manual deve informar o id em `google.drive_folder`. Registrar no ADR da pasta de DR e em `docs/deploy.md`.

**Onde persistir.** Alternativas avaliadas: (a) campo novo em `google_token`: rejeitada, porque `GoogleTokenRepository.Save` é um replace do registro e a reautorização do OAuth apagaria o id; (b) coleção genérica de settings: rejeitada por ser abstração sem uso além deste caso; (c) **coleção singleton dedicada `drive_dr`: escolhida**, ciclo de vida independente do token.

**Migration `0050_drive_dr.go`** (`internal/platform/pb/migrations/`, próxima da numeração `0010..0040`): coleção base `drive_dr` com `folder_id` (`textField`, obrigatório), `folder_name` (`textField`), `timestamps`. Regras nulas (somente superuser), como as demais. `down` = `dropCollections(app, "drive_dr")`. O id da pasta não é segredo (fica em claro).

**Porta e adapter.**

```go
// internal/app/ports.go
type DriveFolderRepository interface {
    Load(ctx context.Context) (folderID string, err error) // ErrNotFound se não houver
    Save(ctx context.Context, folderID string) error       // cria-ou-atualiza (singleton)
}
```

Adapter `internal/platform/pb/drive_folder_repo.go` (`NewDriveFolderRepository(a core.App)`), singleton lógico no estilo de `googleTokenRepo` (`FindRecordsByFilter("drive_dr", "", "created_at", 1, 0)`). Teste de adapter no padrão dos demais (`tests.NewTestApp`).

**Resolver (`internal/drivesync/folder.go`).**

```go
type FolderClient interface { // implementada pelo adapter do wiring sobre google.DriveClient
    FindFolder(ctx context.Context, name string, appProps map[string]string) (string, bool, error)
    CreateFolder(ctx context.Context, name string, appProps map[string]string) (string, error)
    GetFolder(ctx context.Context, id string) (trashed bool, err error) // ErrFolderNotFound em 404
}
type FolderStore interface { // satisfeita por um adapter fino sobre app.DriveFolderRepository
    Load(ctx context.Context) (string, error) // ErrNoStoredFolder se não houver registro
    Save(ctx context.Context, id string) error
}

// Sentinelas LOCAIS do drivesync (o pacote não importa integration/google nem app).
var (
    // ErrFolderNotFound: o Drive respondeu 404 para a pasta (ou para o pai no upload).
    ErrFolderNotFound = errors.New("drivesync: pasta não encontrada")
    // ErrNoStoredFolder: FolderStore.Load não tem id persistido ("não existe").
    ErrNoStoredFolder = errors.New("drivesync: nenhuma pasta persistida")
)
type FolderResolver struct { /* configured string; client; store; mu sync.Mutex; cached string */ }

func NewFolderResolver(configured string, c FolderClient, s FolderStore) *FolderResolver
func (r *FolderResolver) ID(ctx context.Context) (string, error)
func (r *FolderResolver) Invalidate(staleID string) // descarta o cache em memória; ignorado se veio do config
```

Sem `Delete` no `FolderStore`: após `Invalidate`, a próxima `ID` **pula o passo 2** (id persistido) e vai direto à busca (passo 3) e, se preciso, à criação (passo 4); o `Save` do passo 3/4 sobrescreve o registro singleton, então o id obsoleto some sem remoção explícita.

**Revalidação do id persistido (independe do spike abaixo).** Quando o id vem de `drive_dr` (passo 2) e ainda não foi validado neste processo, `ID` faz uma checagem leve `files.get?fields=id,trashed` (novo método `GetFolder(ctx, id) (trashed bool, err error)` em `FolderClient`/`DriveClient`; 404 vira `google.ErrDriveNotFound`, traduzido para `drivesync.ErrFolderNotFound` pelo adapter; ver abaixo). Se `trashed=true` ou `errors.Is(err, ErrFolderNotFound)`, trata como obsoleto: `Invalidate` e segue para busca e criação. Resultado válido é guardado em `validated bool` no resolver, então a checagem custa **uma** chamada por processo, não por backup. O id vindo do config nunca é revalidado nem substituído. Isso cobre o caso mais comum de "apagar à mão" (mandar para a lixeira) sem depender de como o Drive reage a upload com pai na lixeira.

O `mu` serializa resoluções concorrentes (um processo, backups sequenciais; o mutex só evita duas criações se dois backups se sobrepuserem). `driveUploaderAdapter` (em `wiring.go`) deixa de ter `folderID string` e passa a ter `resolver *FolderResolver`; no `Upload`: `id, err := resolver.ID(ctx)` e então `client.Upload`.

**Pasta apagada à mão (id persistido obsoleto).** Dois casos distintos. (a) **Pasta na lixeira:** **SUPOSIÇÃO NÃO VERIFICADA (NÃO VERIFICÁVEL LOCALMENTE: exige credencial real)** se o Drive rejeita ou aceita upload com pai na lixeira ou apagado (se aceitar, os snapshots cairiam numa pasta lixeira em silêncio, reabrindo R3.3); por isso a detecção não depende disso: é a revalidação `trashed` do `FolderResolver` acima. **Spike manual (F2, credencial de dev):** criar a pasta, mandá-la para a lixeira, fazer upload e anotar o status; o resultado vai para o ADR, mas o design já está protegido nos dois desfechos. (b) **Pasta excluída definitivamente ou conta trocada:** o 404 do pai é o esperado (evidência da investigação só cobre 404 para id inexistente) e `restDriveClient.Upload` o mapeia para `google.ErrDriveNotFound` (erro sentinela). **Tradução de sentinelas (fronteira de pacote).** `google.ErrDriveNotFound` **ainda não existe** (verificado: `internal/integration/google/drive.go` não define sentinela nem trata 404; é código novo desta onda). Como `drivesync` não pode importar `integration/google`, o sentinela que o resolver enxerga é `drivesync.ErrFolderNotFound` (`internal/drivesync/folder.go`). O `driveUploaderAdapter` (wiring) traduz `errors.Is(err, google.ErrDriveNotFound)` para `drivesync.ErrFolderNotFound` (com `%w` mantendo a causa) em `GetFolder` e em `Upload`; o resolver e o adapter só testam `errors.Is(err, drivesync.ErrFolderNotFound)`. Para "não existe" no store, `FolderStore.Load` devolve `drivesync.ErrNoStoredFolder`: o wiring envolve `app.DriveFolderRepository` num adapter que converte `app.ErrNotFound` (`internal/app/errors.go`) nesse sentinela, de modo que o `drivesync` não importa `app`. Qualquer outro erro de `Load` é fatal para a resolução (propaga, `Error`). O adapter, ao receber `ErrFolderNotFound` com id **persistido**, chama `resolver.Invalidate(id)` e refaz a resolução **uma única vez** (busca, depois criação) e repete o upload. Com id vindo do **config**, o 404 não dispara criação: vira erro observável (o operador definiu a pasta). Também cobre troca de conta Google (com `drive.file`, a pasta de outra conta é invisível: 404 → recria na conta nova).

**`wireDriveSync` (R3.4).** Deixa de exigir `DriveFolder`; liga com `client_id` + `client_secret` + refresh token. Assinatura do seam (final em [§3.1](#31-contrato-interno-do-drivesync)): `driveSyncWirer(a core.App, kp *crypto.MapKeyProvider, googleAuth *app.GoogleAuthService, folders app.DriveFolderRepository, obs drivesync.Observer, st *drivesync.Status)`. Hoje nenhum teste usa o seam (verificado por grep), então a mudança de assinatura é segura. `buildServices` cria o `pb.NewDriveFolderRepository(a)` e o passa. A mensagem de desativação passa a citar só `client_id/client_secret` e refresh token.

**Degradação observável (R3.3).** O silêncio some em três níveis:
- Log `Error` com `backup`, `etapa` (`resolver_pasta`, `upload`, `retencao`) e `erro` (sem segredo).
- Métrica `psicoman_drive_espelhamento_total{resultado="erro"}` e `psicoman_drive_dr_pronto` (R8.6).
- `/readyz` ganha a chave `checks.drive_dr` com `desativado | ok | degradado`, **sem** alterar `ready` (igual ao `google`: o app opera sem DR). Fonte: `drivesync.Status` (contrato em [§3.1](#31-contrato-interno-do-drivesync)), exposto em `service.Services.DriveStatus *drivesync.Status` e lido por `readyz` em `internal/api/health.go`. `degradado` = último espelhamento falhou; `ok` = último ok **ou** configurado sem tentativa ainda; `desativado` = sem credenciais (`New` registra `SetConfigured(store != nil)`; tabela completa em §3.1).

### 3.1 Contrato interno do `drivesync`

Hoje (verificado em `internal/drivesync/drivesync.go`) o `Syncer` conhece só `SnapshotUploader.Upload(ctx, name, content) (string, error)` e `BlobSealer`, com `New(uploader, sealer)`. Esta seção fecha o que R2 (retenção), R3 (pasta, degradação) e R8.6 (métricas) exigem dele. O pacote **não** importa `integration/google` nem `client_golang`; tudo entra por interfaces locais.

```go
// internal/drivesync/store.go

// File é a visão mínima de um snapshot remoto (independente do tipo do google).
type File struct {
    ID      string
    Name    string
    Created time.Time
}

// SnapshotStore substitui SnapshotUploader no Syncer. Satisfeita pelo
// driveUploaderAdapter (wiring), que já conhece a pasta de DR (FolderResolver)
// e o AppProps {"psicoman_kind":"snapshot"}: o drivesync não sabe de pasta nem de props.
type SnapshotStore interface {
    SnapshotUploader                                   // Upload(ctx, name, content) (id, err)
    List(ctx context.Context) ([]File, error)          // só snapshots do app na pasta de DR, mais recentes primeiro
    Trash(ctx context.Context, fileID string) error    // lixeira (reversível)
}

// Observer recebe os fatos que viram métrica. Implementada em platform/metrics
// (adapter de borda); NopObserver{} nos testes e quando métricas estão desligadas.
type Observer interface {
    Backup(destino string, ok bool, d time.Duration, sizeBytes int64) // destino: "local" | "drive"
    Mirror(resultado string)                                          // "ok" | "erro" | "desativado"
    Retention(kept int)                                               // snapshots mantidos após a limpeza
    RetentionFailure(etapa string)                                    // "listar" | "lixeira"
}
// Nota: não há métodos para drive_dr_pronto nem para o timestamp do último sucesso.
// Ambos são DERIVADOS no adapter de platform/metrics: backup_ultimo_sucesso_timestamp_segundos{destino}
// = relógio no momento de Backup(destino, ok=true, ...); drive_dr_pronto = 1 quando o último
// Mirror foi "ok" (0 em "erro"/"desativado"). O implementador não deve acrescentar métodos ao Observer.
type NopObserver struct{}

// Status guarda o último resultado do espelhamento (atomic.Value); alimenta /readyz.
type Status struct{ /* configured atomic.Bool; last atomic.Value (string) */ }
func (s *Status) SetConfigured(b bool) // chamado por New(...) com store != nil
func (s *Status) Set(res string)       // "ok" | "erro" (último espelhamento)
func (s *Status) State() string        // tabela abaixo

// Tabela de State() (avaliada em ordem):
//   !configured                 -> "desativado"   (valor zero de Status: nunca configurado)
//   configured, sem tentativa   -> "ok"
//   último Set("erro")          -> "degradado"
//   último Set("ok")            -> "ok"
// O valor zero de Status (&Status{} ou nil-safe via métodos) é "desativado".

// Construtor (substitui New(uploader, sealer)). Registra o estado
// de configuração: New chama st.SetConfigured(store != nil) (st nil vira &Status{}).
func New(store SnapshotStore, sealer BlobSealer, retention int, obs Observer, st *Status) *Syncer
```

Regras de comportamento (todas dentro do hook `OnBackupCreate` já existente):

- **Quem mede o backup local (`destino="local"`).** O próprio `Syncer`, no hook: `t0 := time.Now()` antes de `e.Next()`; depois de `e.Next()` bem-sucedido, tamanho = `Attributes(name).Size` do `NewBackupsFilesystem` (o mesmo filesystem que o `mirror` abre; **VERIFICADO:** em PocketBase v0.40.6 o método é `func (s *System) Attributes(fileKey string) (*blob.Attributes, error)` em `tools/filesystem/filesystem.go:168`, e `blob.Attributes.Size` é `int64` (o próprio `GetFile` usa `attrs.Size`, l.211); a alternativa de contar os bytes de `io.ReadAll` do `mirror` fica descartada, mas serve de plano B). Chama `obs.Backup("local", true, since(t0), size)`; se `e.Next()` retorna erro, `obs.Backup("local", false, since(t0), 0)` e o erro segue propagado (comportamento atual). A medição acontece **antes** do mirror, independentemente de haver `store`.
- **Backup `destino="drive"`.** `mirror` mede de `resolver`→`upload` (duração) e o tamanho do blob **cifrado** enviado; chama `obs.Backup("drive", ok, d, len(sealed))`. Em seguida `obs.Mirror("ok"|"erro")` e `st.Set(...)`. Sem `store` (credenciais ausentes): `obs.Mirror("desativado")` e `Warn` (como hoje); `st.State()` devolve `desativado` porque `New` registrou `SetConfigured(false)`. Teste unitário em `drivesync_test.go` cobre a tabela de `State()` (zero value, configurado sem tentativa, `erro`, `ok`, e `ok` depois de `erro`).
- **Retenção e `drive_snapshots_mantidos`.** Só após upload ok e `retention > 0`: `store.List` → `selectToTrash` → `store.Trash` por item. Ao final, `obs.Retention(len(files) - len(trashadosComSucesso))` (a métrica de gauge **não** é atualizada quando `List` falha; `obs.RetentionFailure("listar")`). Erro de `Trash` em um item: `obs.RetentionFailure("lixeira")`, `Warn`, continua nos demais. Nada disso altera `st` nem o resultado do espelhamento.
- **`Status` e `/readyz`.** `buildServices` cria um único `*drivesync.Status` e o passa a `wireDriveSync` (que o entrega a `drivesync.New`) e a `Services.DriveStatus`. Com `Services.DriveStatus == nil` (testes que montam `Services` à mão), `readyz` omite `checks.drive_dr` em vez de panicar.
- **`driveUploaderAdapter`** (em `wiring.go`) passa a satisfazer `SnapshotStore` **e** `FolderClient`: `Upload` resolve a pasta (`resolver.ID`), repete uma vez em `ErrDriveNotFound` ([§3](#3-r3--pasta-de-dr-criada-automaticamente)) e grava `AppProps`; `List` chama `google.DriveClient.List` com `FolderID` resolvido e converte `google.DriveFile` para `drivesync.File`; `Trash` delega. A conversão de tipos mora no adapter, não no `drivesync`.
- **Seam de teste.** `driveSyncWirer` ganha o parâmetro `st *drivesync.Status` (a assinatura final é `driveSyncWirer(a core.App, kp *crypto.MapKeyProvider, googleAuth *app.GoogleAuthService, folders app.DriveFolderRepository, obs drivesync.Observer, st *drivesync.Status)`). Os testes do `drivesync` usam um `fakeStore` (Upload/List/Trash em memória), um `recordingObserver` e `Status` real.

### 3.2 Fluxo e testes de R3

**Fluxo do primeiro backup.** `OnBackupCreate` → `e.Next()` (backup local) → `mirror`: lê o zip → cifra → `resolver.ID` (cria a pasta, persiste) → `Upload` com `AppProps{psicoman_kind:snapshot}` → retenção → `Status.Set(ok)` → métricas. Segundo backup: `resolver.ID` devolve do cache/`drive_dr` sem chamar a API.

**Impacto em testes (todos com `FakeDriveClient`, nenhum toca a API real).**
- Unit do `FolderResolver`: config vence e não chama o client; persistido é reusado (e validado uma vez por processo); **persistido na lixeira (`trashed=true`) ou 404 no `GetFolder` → busca/criação e novo id salvo**; `Invalidate` seguido de `ID` não relê o persistido; busca acha e persiste; criação persiste; dois `ID` seguidos = 1 `CreateFolder`; falha de `CreateFolder` devolve erro e não persiste; `Invalidate` com id do config não recria.
- `drivesync_test.go`: pasta obsoleta (`ErrFolderNotFound`) → recria uma vez e repete o upload; segunda falha não faz laço.
- `drive_test.go` com servidor `httptest`: `CreateFolder`, `FindFolder`, `List`, `Trash` e o `appProperties` no multipart; novas vars de pacote `driveFilesURL = "https://www.googleapis.com/drive/v3/files"` (metadados), análogas a `driveUploadURL`.
- E2E (`test/e2e/backup_test.go`): o binário não aceita injetar o fake; a cobertura E2E fica **em processo** em `internal/api` reatribuindo o seam `driveSyncWirer` com `FakeDriveClient`, disparando `POST /v1/admin/backup` e conferindo: 1 `CreateFolder`, 1 `Upload` com `FolderID` da pasta criada, registro em `drive_dr`, segundo backup sem nova criação, `checks.drive_dr` no `/readyz`. O E2E existente (`TestBackupSemGoogleNao500`) permanece.

**Docs.** `docs/deploy.md` (seção `google.*`: `drive_folder` opcional; criação automática; nome "Psicoman DR"; renomear/mover a pasta é seguro, apagar recria), `docs/architecture.md §0.10`, `config.example.yaml` e `deploy/config.docker.yaml` (comentar `drive_folder` como opcional), ADR (onde persiste o id e a ordem de resolução).

---

## 4. R4 — golangci-lint reativado

**Estado verificado.** `golangci-lint v1.64.8` (compilado com go1.27.1) não decodifica o export data do Go 1.27; `.golangci.yml` está no formato v1; o alvo `lint` do Makefile roda um probe, pula na incompatibilidade, engole falhas com `|| echo` e termina com `@true`. O CI não instala o linter.

**Decisão.** Migrar para **golangci-lint v2.15.0** (última v2 encontrada; `go list -m -versions` na investigação), instalada com `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`. Instalar **compilando com o toolchain local** é o ponto: binários pré-compilados (inclusive a action oficial `golangci-lint-action` em modo binário) foram buildados com Go mais velho e repetem exatamente o erro de export data. **VERIFICADO (2026-11-05):** `go list -m -versions github.com/golangci/golangci-lint/v2` lista `v2.15.0` (a mais recente); `GOBIN=/tmp/gobin go install ...@v2.15.0` compila com go1.27.1 e `golangci-lint version` imprime `has version 2.15.0 built with go1.27.1`; `golangci-lint run ./...` **decodifica o export data do Go 1.27** (nenhuma mensagem de export data/typecheck; `staticcheck`, `unused` e `revive` produziram achados reais). **Números da triagem (config migrada em cópia, `.golangci.yml` real intacto): 185 achados** (`revive` 127, `misspell` 43, `staticcheck` 15). Com o rascunho abaixo (`default: none`, sem os presets de exclusão que o `migrate` acrescenta) são **195** (`errcheck` 8, `misspell` 43, `revive` 129, `staticcheck` 15) e o `config verify` passa. Composição: `revive` é quase todo `error-strings` (≈60, mensagens capitalizadas em PT-BR) e `unused-parameter` (≈56); `misspell` (43) são **falsos positivos**, porque o dicionário padrão trata palavras em português (`operacional`, `autor`, `independente`) como erro de inglês; `errcheck` (8) são `Close()` de `fsys`/`Body`/`reader` sem checagem; `staticcheck` traz `SA1019` (`ResetBootstrapState` e `parser.ParseDir` deprecados), `QF1012` em `platform/pdf` e um `S1017` em `drivesync.go:110`. **Implicação:** 195 > 100, então a regra de baseline se aplica à letra; mas desligar com justificativa `misspell` (PT-BR), `revive:error-strings` e `revive:unused-parameter` derruba o resto para ≈35 achados, que se corrigem sem baseline. A tarefa 1.5 decide pelo número **depois** de configurar essas três exclusões. O spike continua no plano (tarefa 1.1) só para **reconfirmar rapidamente**. Se, ao reconfirmar, falhar, a tarefa para e o Rafael decide (esperar release, ou rodar o linter com `GOTOOLCHAIN` do 1.26 na ferramenta, o que perde a análise de código que use features do 1.27).

**Migração de `.golangci.yml`** (`golangci-lint migrate` gera o rascunho, revisado à mão):

```yaml
version: "2"
run:
  timeout: 5m
  tests: true
linters:
  default: none
  enable: [errcheck, govet, ineffassign, staticcheck, unused, misspell, revive]
  exclusions:
    rules:
      - path: _test\.go
        linters: [errcheck]   # Close()/Body em testes
formatters:
  enable: [gofmt]
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

(`gofmt` vira *formatter*; `staticcheck` absorve gosimple/stylecheck; a chave de exclusão muda de `issues.exclude-rules` para `linters.exclusions.rules`. Conferir o schema exato com `golangci-lint config verify`.) **VERIFICADO (2026-11-05):** o rascunho acima passa no `config verify` da v2.15.0. O `golangci-lint migrate` (em cópia) gera algo diferente do rascunho: mantém o conjunto padrão de linters (sem `default: none`), acrescenta `exclusions.presets` (`comments`, `common-false-positives`, `legacy`, `std-error-handling`, que silenciam os 8 `errcheck` de `Close()`), descarta `run.timeout` (aviso "ignored"; na v2 o timeout é desligado por padrão) e não migra os comentários. Decidir na tarefa 1.2 qual dos dois formatos adotar; a recomendação é o rascunho explícito (`default: none`), ficando os `Close()` tratados na triagem (2.1) em vez de escondidos por preset. Na triagem, desligar com comentário justificando: `misspell` (falsos positivos em português), `revive:error-strings` e `revive:unused-parameter`.

**Makefile.**
- `GOLANGCI_LINT_VERSION ?= v2.15.0`.
- Novo alvo `tools`: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)`.
- `lint: fmt-check vet`, depois: se `golangci-lint` não existe, **falha** (`exit 1`) com a mensagem "rode `make tools`"; se a versão instalada difere da fixada, **falha** com mensagem; senão `golangci-lint run ./...` **sem** `|| echo`, **sem** `@true`, **sem** probe. O `timeout 180` externo sai (o `run.timeout: 5m` do próprio YAML governa).
- Escape explícito e barulhento: `make check SKIP_LINT=1` imprime `!! LINT PULADO POR PEDIDO EXPLÍCITO` e sai 0. Não há mais skip automático. **Recomendação de design** (mantém a ergonomia local sem reintroduzir o skip silencioso).
- Comentário do cabeçalho do Makefile ("lint advisory") atualizado.

**CI** (`.github/workflows/ci.yml`): no job `check`, antes de `make check`, passo `make tools` (e `$(go env GOPATH)/bin` no `PATH`). O cache do `setup-go` cobre o build do linter nas execuções seguintes.

**Fase de triagem.** O volume de achados novos é desconhecido até o spike. Regras: (1) corrigir o que for bug real (errcheck, ineffassign, staticcheck SA); (2) estilo de baixo valor (`revive` ruidoso) pode ser desligado por regra no YAML com comentário justificando; (3) supressão pontual só com `//nolint:<linter> // <justificativa>` (o `nolintlint` não está habilitado, então a regra é de revisão); (4) correções em massa vão em **commits separados** do trabalho funcional, na própria fase R4, para o diff funcional das demais fases não ser contaminado. Se o volume passar de ~100 achados, a fase vira duas: "ativar com baseline" (`issues.new-from-rev` apontando para o commit anterior à fase, para o gate exigir só código novo) e "quitar a dívida" em tarefa seguinte; registrar a escolha no ADR.

**Testes/validação.** Não há teste automatizado do Makefile; validação manual documentada na tarefa: `make lint` com o linter ausente falha, com versão errada falha, com `SKIP_LINT=1` avisa e passa, e com tudo certo roda de verdade (injetar um `_ = x` com `ineffassign` temporário para provar que quebra o gate, depois remover).

**Docs.** `docs/deploy.md`/README (instalar com `make tools`), `docs/decisions.md` (ADR curto: linter v2, fim do modo advisory).

---

## 5. R5 — Testes da auditoria clínica

Detalhados em [R1](#1-r1--auditoria-lgpd-no-caminho-clínico). Resumo da cobertura mínima: unit (4 casos de ação + não-vazamento + `audit==nil`), E2E de contrato com o binário real lendo `GET /v1/admin/audit-log`. A segurança de acesso exige permitido **e** negado: o E2E também confirma `401` em `GET /v1/app/patients/{id}` sem `X-Pangolin-User` e que **nenhuma** entrada de auditoria é gravada nesse caso. Nenhum `t.Skip`.

---

## 6. R7.4 — Ator do audit pela identidade real

(Na ordem de implementação, esta seção vem **antes** de R1: R1 usa o ator do contexto.)

**Decisão.** Propagar o ator por `context.Context`, não por parâmetro. Evita alterar ~15 assinaturas, resolve R1 e R7.4 juntos e funciona também para gravações disparadas por hooks síncronos dentro do mesmo request (ex.: débito gerado ao finalizar sessão).

```go
// internal/domain/actor.go (stdlib apenas; domain continua sem PocketBase)
const ActorSystem = "sistema"
type actorKey struct{}
func WithActor(ctx context.Context, actor string) context.Context
func ActorFrom(ctx context.Context) string // "" se ausente
```

`AuditService.Record` resolve o ator efetivo: se o `actor` recebido é `""` ou `domain.ActorSystem`, usa `domain.ActorFrom(ctx)`; se ainda vazio, `domain.ActorSystem`. Assim as 11 chamadas existentes com `"sistema"` passam a registrar a identidade real quando há request, **sem editar cada uma** (cada uma pode migrar para `""` opcionalmente; recomendado deixar como está para diff mínimo). Ações sem request (job de backup, hooks de boot) continuam `"sistema"`.

**Middleware.** `httpx.PropagateActor(e)`: lê `ActorIdentity(e)` e faz `e.Request = e.Request.WithContext(domain.WithActor(e.Request.Context(), id))`. Ligado nos grupos `/v1/app` e `/v1/admin` **depois** de `RequirePangolinIdentity` (`appGrp.BindFunc(httpx.RequirePangolinIdentity(...), httpx.PropagateActor)`). Os handlers derivam o contexto de `e.Request.Context()` (`reqCtx`), então o ator chega ao service. `httpx` passa a importar `internal/domain` (permitido; `domain` não importa `httpx`).

**Modo dev.** `RequirePangolinIdentity` já grava a identidade dev (`pangolin.dev_identity`) quando o header falta, e o header real vence; o ator herda isso sem código extra. O `audit_log` em dev mostra o valor configurado. Nenhuma marcação especial, e a documentação do `docs/deploy.md` avisa que o ator em modo dev não é identidade verificada.

**TODO.** Remover o comentário `// TODO Fase 5: extrair o autor do AuthAppStub...` em `internal/api/app/sessions.go` (~l.127). O valor `scheduling.CancelByTherapist` está correto em single-tenant (única identidade em `/v1/app`) e fica; o comentário é trocado por uma linha explicando isso.

**Portal.** Fora do escopo de R7.4: ações de paciente não passam pelo Pangolin e seguem `"sistema"`; registrar como limitação no ADR.

**Privacidade.** O ator é o e-mail/handle do terapeuta, gravado no `audit_log` (interno). **Nunca** vai a rótulo de métrica (R8.9.1).

**Testes.** Unit: `Record` com ctx com ator grava o ator; sem ator grava `"sistema"`; `"sistema"` explícito com ctx com ator troca pelo ator. `httpx`: `PropagateActor` coloca o ator no ctx (padrão de `auth_test.go`). E2E: a mesma chamada de R1 confere `actor == X-Pangolin-User`; um `POST .../sessions/{id}/cancel` confere o ator no `audit_log` (`session.cancel`).

**Docs.** `docs/architecture.md` (seção de auditoria), ADR (propagação por contexto; portal fora).

---

## 7. R7.1 — Rate limit por e-mail

**Decisão.** Limitador **em memória**, janela fixa por chave, stdlib apenas (sem `golang.org/x/time/rate`: janela fixa é suficiente e evita dependência). As duas camadas valem: o nginx segue limitando por IP (30 r/min, `deploy/nginx/portal.conf`, inalterado) e o app limita por identidade.

```go
// internal/api/httpx/ratelimit.go
type LimiterOptions struct {
    Limit    int
    Window   time.Duration
    MaxKeys  int
    Scope    string                // enum fechado: ScopeCadastro | ScopeAgendamento; vira o rótulo "escopo" da métrica e o prefixo de EmailKey
    Log      *slog.Logger          // Warn de saturação (máx. 1/min); nil = slog.Default()
    OnReject func(scope string)    // chamado em todo 429 (estouro e saturação); ligado ao recorder no wiring; nil = no-op
    Clock    func() time.Time      // nil = time.Now
}
const (
    ScopeCadastro    = "cadastro"
    ScopeAgendamento = "agendamento"
)
type Limiter struct { /* mu sync.Mutex; entries map[string]*window; opts LimiterOptions; calls int; lastWarn time.Time */ }
func NewLimiter(o LimiterOptions) *Limiter
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration)
func EmailKey(scope, email string) string   // sha256(scope + ":" + lower(trim(email))) em hex
```

`Scope` é um **único enum** usado em três lugares: prefixo da chave (`EmailKey(l.Scope(), ...)`), rótulo `escopo` de `psicoman_rate_limit_rejeicoes_total` ([§10.3](#103-famílias-de-métricas-prefixo-psicoman_)) e texto do log. Os valores `register`/`appt` **não** existem mais. `Allow` devolve `ok=false` tanto no estouro da janela quanto na saturação da tabela, e nos dois casos chama `OnReject(scope)` antes de retornar; `ok=true` nunca chama. No wiring, `portal.Mount` recebe `RateLimitHooks{OnReject func(scope string)}` (ou o recorder `httpx.HTTPRecorder`, que ganha `RateLimited(scope string)`; [§10.9](#109-assinaturas-go-de-r8)) e monta `NewLimiter(LimiterOptions{..., OnReject: obs.RateLimited})`. A métrica fica no recorder HTTP (e não em `app.Metrics`) porque o limitador mora em `httpx`/`api/portal`, camada de borda que já fala com `HTTPRecorder`; `app.Metrics` continua só com eventos de negócio.

- **Janela fixa** por chave (`count`, `start`); expira quando `now - start >= window`.
- **Limpeza sem goroutine**: a cada `Allow`, se `len(entries) > maxKeys/2` ou a cada 256 chamadas, varre e remove janelas expiradas. Se ainda `>= maxKeys` (ataque com e-mails distintos), nova chave é **recusada com 429** em vez de crescer sem limite (fail-closed de memória). Uma chave **já existente** dentro do limite continua sendo atendida com a tabela cheia.
- **Dimensionamento: `maxKeys = 10000`**, constante nomeada `maxLimiterKeys` em `internal/api/portal` (um `Limiter` por escopo, então até 10000 chaves cada; cada entrada ocupa ~100 bytes, ≈1 MB no pior caso). Justificativa: o nginx limita a 30 r/min por IP (burst 20), o que dá no máximo 1800 chaves novas por hora por IP, e a janela é de 1 h; um único IP, portanto, não enche a tabela. São necessários mais de 5 IPs distintos sustentando o ataque por uma hora. **Risco aceito (a registrar no ADR): saturação por botnet** nega cadastros novos por até 1 h (sem perda de dados, sem afetar pacientes já cadastrados, nem o agendamento, que usa outra tabela). Mitigação: ao saturar, `Warn` com a contagem de chaves (no máximo 1 por minuto, `rate-limited`) e a métrica `psicoman_rate_limit_rejeicoes_total{escopo}` expõe o pico; o alerta de rejeições em [§10.6](#106-política-de-rede-recomendada-recomendação-de-design) o evidencia.
- **Anti-enumeração.** A chave é o hash do e-mail normalizado; **toda tentativa conta, exista ou não o e-mail**, e o 429 é decidido **antes** de qualquer consulta ao banco (`RegisterFromPortal`). A resposta 429 é idêntica para qualquer e-mail. Nunca guardar nem logar o e-mail; só o hash truncado (8 hex) no log `Warn` de bloqueio.
- **Valores (DECIDIDO):** `register`: 5 tentativas por hora por e-mail; `appointment-requests`: 10 por hora por paciente. São constantes nomeadas em `internal/api/portal` (`registerLimitPerHour`, etc.). Sem config nova nesta onda (YAGNI); se a terapeuta precisar, vira config depois.

**Onde aplicar.**
- `POST /v1/portal/register`: dentro do handler, **logo após `BindBody` e ANTES da validação de senha** (`len(body.Password) < minPortalPassword`, hoje em `me.go` logo depois do bind; portanto tentativas com senha fraca, que devolvem 422 `senha_fraca`, também contam no limite) e antes de `Patients.RegisterFromPortal` (o e-mail só existe depois do bind; o `RereadableReadCloser` do router não é necessário). Chave = `EmailKey(ScopeCadastro, body.Email)`. E-mail vazio: sem chave, segue para a validação normal (que devolve 422).
- `POST /v1/portal/appointment-requests`: no handler, após `resolvePatient`. **Correção factual** (ver notas): a rota está dentro do gate de aprovação e a identidade vem da sessão, então a chave é `EmailKey(ScopeAgendamento, patientID)` (hash do `patient_id`), não um e-mail do corpo.

**Resposta.** `httpx.RespondError(e, 429, "limite_excedido", "Muitas tentativas. Tente novamente mais tarde.", nil)` + header `Retry-After` (segundos, arredondado para cima). Envelope PT-BR padrão. O limitador é instanciado no `Handlers` do portal (campo `limiter`), criado em `portal.Mount`; relógio injetável para teste.

**Limitação conhecida (não corrigida aqui).** O `register` hoje devolve `409` quando o e-mail já existe (conflito do `CreateLogin`/`RegisterFromPortal`), o que é um canal de enumeração. O rate limit limita a taxa de sondagem, mas não o fecha; fechar exige decisão de produto (resposta uniforme `201`). Registrar no ADR como risco aceito e abrir como backlog. Também fora do escopo: o login nativo `/api/collections/patient_user/auth-with-password` não passa por este limitador (e `portal.conf` bloqueia `/api/` na borda pública); registrar como achado para o Rafael.

**Testes.** Unit do `Limiter` com relógio fake (dentro do limite, estouro, expiração da janela, limpeza) e de saturação (usar `maxKeys` pequeno, p. ex. 3, no construtor): com a tabela cheia, **chave nova recebe 429** e **chave existente dentro do limite continua atendida**; expirada a janela, a chave nova volta a ser aceita; o `Warn` de saturação sai uma vez por minuto (logger fake); `OnReject` é chamado exatamente uma vez em 429 por estouro e em 429 por saturação, com o `Scope` correto, e **não** é chamado quando `Allow` devolve `ok`. E2E em `test/e2e/portal_test.go`: 5 cadastros com o mesmo e-mail (variando a caixa/espaços) → o 6º recebe `429` com envelope e `Retry-After`; e-mail diferente continua `2xx/4xx` normal (permitido e negado); o corpo do 429 não contém o e-mail. Para agendamento: E2E em processo com paciente aprovado. Métrica `psicoman_rate_limit_rejeicoes_total{escopo}` (R8).

**Docs.** `docs/deploy.md` (duas camadas: nginx por IP, app por e-mail/paciente), `docs/architecture.md`, Swagger: declarar `429` nas duas rotas (o `contract_test.go` compara só rotas, não códigos de resposta; **VERIFICADO**, detalhes abaixo), steering `psicoman-seguranca-lgpd.md` não muda (já pede as duas camadas).

**Swagger e `429` (VERIFICADO, 2026-11-05).** A spec é `internal/api/swagger/openapi.base.json` (embutida por `//go:embed` em `swagger.go`, que só serve a UI e o JSON; **não** há mapa de respostas em Go). O `contract_test.go` tem dois testes: `TestSwaggerBateComAsRotasRegistradas` compara **só** `MÉTODO + caminho` (rotas extraídas de `app/mount.go`, `portal/me.go`, `admin/ops.go` e `api.go` contra os `paths` do JSON) e `TestSwaggerOperacoesBemFormadas` exige por operação `summary`, `responses` **não vazio**, uma única tag coerente com o prefixo e `security` (exceto as rotas abertas, entre elas `/v1/portal/register`). Ele **não** compara códigos de resposta. Para declarar `429`: editar `openapi.base.json`, acrescentando `"429": { "description": "Limite de tentativas excedido (limite_excedido); header Retry-After" }` em `responses` de `POST /v1/portal/register` e `POST /v1/portal/appointment-requests`; o teste continua verde sem outra mudança. As tarefas 5.7 e 6.2 que citam `swagger.go` devem apontar para `openapi.base.json`.

---

## 8. R7.2 — Validação do `state` do OAuth

**Decisão.** Dois mecanismos combinados, ambos obrigatórios:
1. **Store em memória**, `state` de uso único com TTL: garante expiração e consumo único (replay inválido). O processo é único, então memória basta; perder o `state` num restart no meio do consentimento só obriga a repetir o fluxo.
2. **Cookie** `psicoman_oauth_state` com o mesmo valor, `HttpOnly`, `SameSite=Lax` (o callback é navegação top-level GET vinda do Google, e `Lax` a envia), `Path=/v1/admin/google`, `MaxAge=600`, `Secure` quando `X-Forwarded-Proto=https` (ou TLS direto). O `Secure` é **só endurecimento**: a proteção real vem do store de uso único e do `SameSite`; se o proxy não repassar `X-Forwarded-Proto`, o cookie sai sem `Secure` e o fluxo funciona igual. `docs/deploy.md` DEVE documentar que o Pangolin/proxy repassa `X-Forwarded-Proto`. Amarra o fluxo ao **navegador** que iniciou (defesa de CSRF/login CSRF real; o store sozinho valida só "este state foi emitido").

Alternativas descartadas: coleção PocketBase efêmera (migration e adapter para algo que vive 10 minutos e num processo só); só cookie (sem uso único: o cookie poderia ser reapresentado); só store (sem amarra ao navegador).

**Porta e implementação** (`internal/app/oauth_state.go`, stdlib):

```go
type OAuthStateStore interface {
    Issue() (state string)                 // ULID aleatório; registra expiração
    Consume(state string) bool             // true uma única vez, e só se não expirou
}
func NewMemoryOAuthStateStore(clk clock.Clock, ttl time.Duration, maxPending int) *MemoryOAuthStateStore
```

`ttl = 10*time.Minute`, `maxPending = 100` (descarta expirados a cada `Issue`; se cheio, remove o mais antigo). O `state` continua `ulid.New()`: a imprevisibilidade vem do componente aleatório do ULID (80 bits); **Recomendação de design:** trocar para 16 bytes de `crypto/rand` em base64url (128 bits) dentro do store, porque o ULID tem prefixo temporal previsível. Mutex interno.

O store entra em `service.Services` (`OAuthState app.OAuthStateStore`), criado em `buildServices`.

**Fluxo.**
- `GoogleAuthorize`: `state := h.svc.OAuthState.Issue()`; `http.SetCookie` com o valor; redireciona (302) como hoje.
- `GoogleCallback` (ordem das checagens): `error` do Google (422 como hoje, antes de tudo e **limpando** o cookie) → `state` da query vs cookie: ausente em qualquer dos dois, ou diferentes (`subtle.ConstantTimeCompare`) → `400 state_invalido`; `Consume(state)` false (expirado ou reusado) → `400 state_invalido` (mesma mensagem, sem distinguir o motivo); só então `code`, `Configured()` e troca. Em **todos** os caminhos o cookie é expirado (`MaxAge=-1`) na resposta.
- Mensagem PT-BR: "Estado da autorização inválido ou expirado. Inicie a autorização novamente."

**Testes.** Unit do store (emite/consome uma vez, expira, cheio). Em processo (`internal/api/google_oauth_test.go`): fluxo válido (authorize → captura `Set-Cookie` e `state` da `Location` → callback com `Cookie` + `state` → 200); sem cookie; state diferente; state reusado (segundo callback → 400); state expirado (relógio do store injetado via seam do wiring ou construindo o `Services` de teste). **Testes existentes que quebram e devem ser atualizados:** `internal/api/google_oauth_test.go` (~l.80, `state=abc`) e `internal/api/health_test.go` (~l.60, `state=s`) chamam o callback sem passar pelo authorize. O `apiClient.do` aceita headers (usar `Cookie`). E2E real com binário: authorize (302, cookie) + callback sem state → `400` (o Google real não é tocado, falha antes da troca).

**Swagger/docs.** Documentar `400 state_invalido` no callback (conferir contrato); `docs/deploy.md` (o callback precisa do mesmo navegador do authorize; funciona atrás do Pangolin porque o cookie é do host externo); ADR curto "state do OAuth validado".

---

## 9. R6 e R7.3 — Documentação, steering e operação

### R6.1 Steering sem Gmail (escopo ampliado)
- `psicoman-google-api.md`: título passa a `Integração Google (Calendar, Meet, Drive)`; remover o escopo `gmail.send` da lista; remover a seção `## Gmail`; ajustar a nota de IA ("Calendar/Gmail/Drive" → "Calendar/Drive") e a seção Testes (remover `GmailClient`; as interfaces vivem em `internal/integration/google`, não em `internal/app`/`internal/domain`); ajustar a seção Drive conforme [R2](#2-r2--snapshot-inteiro-oficializado-e-retenção) e acrescentar a criação automática da pasta ([R3](#3-r3--pasta-de-dr-criada-automaticamente)). Acrescentar uma linha "Gmail: fora de escopo (ver `docs/requirements.md` §3.7/§5)".
- `psicoman-golang.md:33` ("Calendar/Gmail/Drive") e `psicoman-testes-e2e.md:17` (`GmailClient`; e a referência ao modelo antigo de dois binários "`admin` ou `portal`" → binário único `cmd/psicoman`): corrigir. O steering de testes também ganha a regra do E2E de métricas (R8).
- Verificação: `grep -ri gmail .kiro/steering/` não devolve nada além da linha "fora de escopo".

### R6.2 Docs
- `docs/architecture.md §0.10` (Drive DR): snapshot inteiro, pasta criada automaticamente (ordem de resolução), `drive_dr`, retenção N=14 por lixeira, degradação observável (`/readyz`, métricas). Nova seção de **observabilidade** (R8) e ajuste de `§0.21` ("o `cmd/psicoman` segue escutando em `127.0.0.1:8090`; o listener de métricas é um segundo endereço interno, só se configurado").
- `docs/deploy.md`: `google.drive_folder` opcional, `google.drive_retention`, criação automática, snapshot inteiro, restauração (baixar `.enc`, decifrar com o `BlobSealer`/chave, descompactar), `metrics.addr`, política de rede, checklist de rotação de segredos (R7.3), nginx em duas camadas.
- `docs/stack-local.md`: porta de métricas só em `expose`, nunca `ports`.
- `docs/requirements.md`: requisito não funcional de observabilidade; `docs/release-notes.md`: entrada da onda no fechamento.

### R6.3 Tabela "Status ao vivo"
Em `.kiro/specs/pocketbase-finish/tasks.md`: remover a linha duplicada `10 — Fechamento ⬜ pendente` e manter a `✅ concluída`; marcar `[x]` em `10.1`, `10.2`, `10.3` e anotar em `10.3` "mergeada no PR #4" (**VERIFICADO (2026-11-05):** `git log --oneline` mostra `5aeeabe Merge pull request #4 from RafaelFino/pocketbase-finish`; a tarefa 9.4 reconfirma com o mesmo comando antes de marcar).

### R7.3 Rotação de segredos
Sem código. Tarefa de checklist no fechamento: rotacionar `google.client_secret` (e revogar/refazer o refresh token pelo fluxo `/v1/admin/google/authorize`) e `llm.gemini.api_key` ao fim dos testes; confirmar que `config.dev.yaml` continua git-ignored (`git check-ignore config.dev.yaml`). Documentar em `docs/deploy.md` (seção de segurança) e citar na release note sem valores.

---

## 10. R8 — Métricas ricas

### 10.1 Decisões de arquitetura

- **Registry próprio** (`prometheus.NewRegistry()`), nunca o `DefaultRegisterer`, com `collectors.NewGoCollector()`, `collectors.NewProcessCollector(...)` e um `info` de build. Evita registro global e deixa os testes isolados.
- **Camadas** (regra de dependência preservada):
  - `internal/app/metrics.go`: **porta** `Metrics` (negócio) + `NopMetrics` + tipos fechados (enums). Sem imports de PocketBase nem de Prometheus.
  - `internal/platform/metrics`: implementação Prometheus (importa `client_golang`): `Registry`, coletores, decoradores. É a única camada (junto com `internal/api/metricsserver.go`, que serve o `Handler`) que importa a lib.
  - `internal/api/httpx/metrics.go`: middleware raiz. `httpx` define sua interface `HTTPRecorder` (não importa a lib); `platform/metrics` a implementa.
  - `internal/api/metricsserver.go`: ciclo de vida do listener.
- **Teste de arquitetura:** estender `internal/archtest/arch_test.go` com `TestDomainAppConfigNaoImportamPrometheus` (padrão dos três testes existentes) proibindo `github.com/prometheus/` em `domain`, `app` e `config`.

### 10.2 Porta de negócio

```go
// internal/app/metrics.go
type PatientEvent string  // PatientRegistered, PatientApproved, PatientRejected
type SessionEvent string  // SessionCreated, SessionConfirmed, SessionCanceled, SessionFinished, SessionNoShow
type ApptEvent string     // ApptCreated, ApptConfirmed, ApptRefused
type Result string        // ResultOK, ResultError, ResultConflict

type Metrics interface {
    Patient(ev PatientEvent)
    Session(ev SessionEvent)
    Appointment(ev ApptEvent)
    DebtGenerated()
    PaymentRecorded()
    InvoicePDFGenerated()
    AuditRecorded(action string)   // action: valor de domain.AuditAction* (enum fechado)
    AuditFailed(action string)
    AIRequest(res Result, d time.Duration)
    CalendarCall(op CalendarOp, res Result, d time.Duration) // op: freebusy|create_event
    CalendarDegraded()             // agendou sem evento por falta de credencial
    CipherOp(op CipherOp, res Result) // seal|open
}
type NopMetrics struct{}           // implementa tudo como no-op
```

Fiação: cada serviço relevante ganha `SetMetrics(Metrics)` (padrão de `SetAudit`), default `NopMetrics{}`, para os testes existentes não precisarem mudar: `PatientService` (cadastro/aprovação/recusa), `SessionService`, `AppointmentService`, `BillingService`, `PaymentService`, `InvoiceService`, `AuditService`. **Rótulos só de enums fechados**; `AuditRecorded` recebe um `string` que o service passa de constante de domínio (nunca valor vindo de entrada).

**Instrumentação por decorador (sem tocar os serviços):** Calendar (`app.ConflictChecker` e `app.SessionCalendar`, os dois adapters do `SessionService`), Gemini (`app.AISummarizer`), cifra (`domain.Cipher`) e `drivesync.SnapshotUploader` são embrulhados em `platform/metrics` no wiring. Calendar `CalendarDegraded` sai do ramo "sem credencial" de `wireCalendar` (a sessão agenda sem evento): hoje isso só gera `Warn` no boot, então a contagem por agendamento exige que o `SessionService` informe quando agenda sem `calendar` configurado. **VERIFICADO (2026-11-05):** o ramo é `if s.calendar == nil { ... return nil }` no início de `func (s *SessionService) createCalendarEvent(ctx, sess)` (`internal/app/session_service.go:292-296`, "No-op se sem Calendar"), chamado por `Create` (l.111) e `Schedule` (l.173); é ali que entra `s.metrics.CalendarDegraded()` (campo `metrics Metrics` com `SetMetrics`, default `NopMetrics{}`). **Correção de nomes:** as interfaces reais em `internal/app/ports.go` são `ConflictChecker` (`HasConflict(ctx, startsAt, endsAt) (bool, error)`, operação `freebusy`) e `SessionCalendar` (`CreateEvent(ctx, CalendarEvent) (*CalendarEventResult, error)`, operação `create_event`); **não existe** `CalendarGateway`. O campo do serviço chama-se `calendar` (tipo `SessionCalendar`) e `conflict` (tipo `ConflictChecker`).

**Gauges de fila e de negócio por coleta** (`GaugeFunc`/`Collector`): pacientes pendentes, pedidos de agendamento pendentes (e totais de pacientes por `approval_status`). Porta de leitura `app.QueueStats{ PendingPatients(ctx) (int, error); PendingAppointments(ctx) (int, error) }`, implementada em `platform/pb` com `CountRecords("patient", dbx.HashExp{"approval_status":"pendente"})` e `CountRecords("appointment_request", dbx.HashExp{"status":"pendente"})` (nomes de coleção e campos verificados em `0010_patient_scheduling.go`). O `Collector` customizado chama as fontes com `context.WithTimeout(2s)`, **cacheia o resultado por 15 s** (um scrape agressivo não martela o SQLite) e, em erro, omite a amostra e incrementa `psicoman_metrics_coleta_erros_total{fonte}`.

### 10.3 Famílias de métricas (prefixo `psicoman_`)

Todas as séries abaixo respeitam R8.9: rótulos são enums fechados; nada de id, e-mail, nome, ator-identidade ou texto livre.

| Família | Tipo | Rótulos | Observações |
|---------|------|---------|-------------|
| `http_requisicoes_total` | counter | `ator, rota, metodo, status` | `status` = código numérico |
| `http_duracao_segundos` | histogram | `ator, rota, metodo` | buckets 5ms..10s (`.005,.01,.025,.05,.1,.25,.5,1,2.5,5,10`) |
| `http_em_andamento` | gauge | `ator` | |
| `http_resposta_bytes` | histogram | `ator, rota` | buckets 256B..1MiB |
| `seguranca_negados_total` | counter | `ator, status, motivo` | `status` 401\|403; `motivo` ∈ `sem_identidade, sem_sessao, nao_aprovado, pocketbase, outro` |
| `identidade_dev_ativa` | gauge | | 1 se `pangolin.dev_identity` não vazio |
| `rate_limit_rejeicoes_total` | counter | `escopo` | `cadastro, agendamento` (R7.1) |
| `pacientes_eventos_total` | counter | `evento` | |
| `sessoes_eventos_total` | counter | `evento` | |
| `agendamentos_eventos_total` | counter | `evento` | pedidos de agendamento |
| `debitos_gerados_total`, `pagamentos_registrados_total`, `pdfs_gerados_total` | counter | | |
| `fila_pacientes_pendentes`, `fila_agendamentos_pendentes` | gauge | | por coleta |
| `calendar_chamadas_total` | counter | `op, resultado` | `ok, erro, conflito` |
| `calendar_duracao_segundos` | histogram | `op` | |
| `calendar_degradado_total` | counter | | agendou sem evento |
| `backup_total` | counter | `destino, resultado` | `destino` = `local\|drive` |
| `backup_duracao_segundos` | histogram | `destino` | |
| `backup_tamanho_bytes` | gauge | `destino` | último snapshot |
| `backup_ultimo_sucesso_timestamp_segundos` | gauge | `destino` | base do alerta "backup velho"; 0 até o primeiro sucesso (documentado) |
| `drive_espelhamento_total` | counter | `resultado` | `ok, erro, desativado` |
| `drive_dr_pronto` | gauge | | 1 quando DR ativo e último ciclo ok |
| `drive_snapshots_mantidos` | gauge | | contagem após a retenção; ausente se a listagem falhar |
| `drive_retencao_falhas_total` | counter | `etapa` | `listar, lixeira` |
| `google_oauth_ok` | gauge | | 1 = `ok`, 0 = degradado (mesmo critério do `/readyz`, via `GoogleAuth.Status`) |
| `gemini_chamadas_total`, `gemini_duracao_segundos` | counter, histogram | `resultado` | |
| `gemini_ligada` | gauge | | 1 se há `api_key` |
| `audit_log_total` | counter | `acao` | `acao` = valores de `domain.AuditAction*` |
| `audit_log_falhas_total` | counter | `acao` | |
| `cifra_operacoes_total` | counter | `op, resultado` | `seal\|open`, `ok\|erro` |
| `metrics_coleta_erros_total` | counter | `fonte` | |
| `build_info` | gauge | `versao, go` | `versao` de `debug.ReadBuildInfo`; valor 1 |
| `uptime_segundos` | gauge | | |
| `go_*`, `process_*` | | | coletores padrão (goroutines, memória, GC, fds) |

**Cardinalidade.** `ator` ∈ 4 valores; `rota` limitada ao conjunto de padrões registrados (hoje ~100) + `nao_encontrada`; `metodo` ∈ 7. Pior caso `http_requisicoes_total` ≈ 100 × 5 status × 4 ≈ poucos milhares de séries, aceitável. O `status` é um valor numérico do conjunto emitido pelos handlers.

### 10.4 Middleware HTTP e mapeamento da rota padrão

**Decisão.** Um middleware no **router raiz** do PocketBase (`e.Router.Bind(&hook.Handler[*core.RequestEvent]{Func: httpx.Instrument(rec), Priority: -5000})` dentro do `OnServe`, em `internal/api/api.go`, antes de montar o grupo `/v1`; `BindFunc` não aceita prioridade, por isso `Bind` com `import "github.com/pocketbase/pocketbase/tools/hook"`), com `Priority` **menor que `-1040`** (`-5000`; abaixo do activity logger e do rate limit do PB, acima do `wwwRedirect` `-99999`), para observar também respostas curtas dos middlewares padrão do PB. Como o `BuildMux` registra o catch-all `/` para 404, ele passa pelo middleware raiz: **rotas inexistentes caem em `rota="nao_encontrada"`** (o padrão é `/`), sem cardinalidade. A decisão de raiz (e não do grupo `/v1`) é necessária porque o middleware de `/v1` não vê 404 de rota desconhecida nem as rotas `/api/*` do PB.

**Derivação (função pura `httpx.RouteLabels(pattern string) (ator, rota string)`):**
- `pattern := e.Request.Pattern` (campo `net/http` desde Go 1.23). **VERIFICADO em runtime (2026-11-05)** com spike descartável (teste temporário em `internal/api` reaproveitando `newTestApp`, middleware global via `e.Router.Bind(&hook.Handler[*core.RequestEvent]{Priority: -5000})`, já removido) sobre o mux do PocketBase v0.40.6. Valores observados no middleware global, lidos **depois** de `e.Next()`:

  | Requisição | Resposta | `Request.Pattern` | `e.Status()` |
  |------------|----------|-------------------|--------------|
  | `GET /v1/app/patients/<ulid>` com `X-Pangolin-User` (paciente inexistente) | 404 | `GET /v1/app/patients/{id}` | 404 |
  | `GET /v1/app/patients/<ulid>` sem header | 401 | `GET /v1/app/patients/{id}` | 401 |
  | `GET /v1/naoexiste` | 404 | `/` | 0 (erro ainda não escrito) |
  | `GET /v1/health` | 200 | `GET /v1/health` | 200 |
  | `GET /api/health` | 200 | `GET /api/health` | 200 |
  | `GET /_/`, `GET /metrics`, `POST /v1/health` (método sem rota) | 404 | `/` | 0 |

  Conclusões: (1) o `Pattern` **inclui o prefixo de método** (`GET /v1/...`), então o prefixo precisa ser removido; (2) o `Pattern` já está **populado quando o middleware global roda antes dos middlewares do grupo**: o 401 de `RequirePangolinIdentity` chega com a rota correta; (3) o catch-all 404 vem como `/` (sem método) e o `e.Status()` é `0`, porque o PB escreve o erro depois da cadeia, o que confirma que o status precisa vir de `router.ToApiError(err)` quando `Next()` devolve erro; (4) `/_/` no app de teste cai no catch-all (o Dashboard só é registrado no servidor real; no binário real o padrão dele cai em `infra` por prefixo, sem cardinalidade, e a tarefa 7.1 reconfirma). `RouteLabels` do design **não muda**; a tabela acima vira o caso de teste de `RouteLabels`. Se o padrão vier vazio (qualquer anomalia), `rota="desconhecida"`, nunca o `URL.Path`.
- Remover o prefixo de método (`"GET "`, ou seja, tudo até o primeiro espaço quando o padrão tiver espaço antes de `/`); se o resto for `/`, `rota="nao_encontrada"`.
- `ator`: prefixo `/v1/app/` → `app`; `/v1/portal/` → `portal`; `/v1/admin/` → `admin`; qualquer outro (`/v1/health|livez|readyz|swagger*`, `/api/*`, `/_/*`) → `infra`.

**Status.** Depois de `e.Next()`: se `err == nil`, `status = e.Status()`; se `err != nil` (o PB escreve o erro depois da cadeia, como no 404 do catch-all), `status = router.ToApiError(err).Status`. Status `0` (nada escrito) vira `200`.

**Tamanho da resposta.** O `ResponseWriter` do PB não conta bytes. O middleware troca `e.Response` por um `countingWriter` (embute `http.ResponseWriter`, soma `Write`, expõe `Status()`, `Written()`, `Flush()`/`FlushError()` e `Unwrap()` delegando ao original, para manter `StatusTracker`, SSE de `/api/realtime` e `http.ResponseController`). **VERIFICADO por leitura do código do PB v0.40.6 (2026-11-05):** em `tools/router/router.go`, `getStatus` e `getWritten` fazem um laço de type switch: `StatusTracker`/`WriteTracker` primeiro e, senão, `RWUnwrapper` (`Unwrap() http.ResponseWriter`) para descer ao writer original; `Event.Status()`/`Written()` (`event.go`) usam esses helpers e `Event.Flush()` usa `http.NewResponseController(e.Response)`, que também segue `Unwrap`. Um wrapper que embute `http.ResponseWriter` e expõe `Unwrap()` é, portanto, reconhecido sem implementar `Status()`. Só **não** foi exercitado em runtime (o spike de runtime cobriu o `Pattern`); a tarefa 7.1 reconfirma com um teste rápido, incluindo `FlushError`/SSE. **Plano B se a verificação falhar:** não contar bytes e usar `Content-Length` quando presente; a família `http_resposta_bytes` fica registrada mas só com amostras onde o header existe (documentar). A métrica de tamanho é a de menor valor e não bloqueia o resto de R8.

**Em andamento.** `Inc`/`Dec` do gauge `http_em_andamento{ator}` com `defer`. O `ator` é conhecido antes de `Next()` pelo `URL.Path` apenas para o gauge (prefixo fixo, 4 valores, sem cardinalidade); as demais séries usam o `Pattern` pós-`Next()`.

**Motivo de 401/403.** Os pontos que geram a negação gravam o motivo no store do evento: `httpx.SetDenyReason(e, reason)` chamado em `RequirePangolinIdentity` (`sem_identidade`), `portal.RequireApproved` (`sem_sessao` / `nao_aprovado`), `portal.resolvePatient` (`sem_sessao`). O middleware lê com `httpx.DenyReason(e)` após `Next()`; 401/403 sem motivo (rotas nativas do PB) vira `pocketbase` para `/api/*` ou `outro`. As constantes são um conjunto fechado em `httpx`. Isso mantém `httpx` sem importar a lib e sem importar métricas nos handlers.

**Falha de métrica nunca vira 500 (R8.10).** O corpo do middleware pós-`Next()` roda dentro de `defer func(){ if r := recover(); r != nil { log Debug } }()`; erros do recorder são engolidos e logados em `Debug` (nível baixo). O middleware **nunca** altera o `err` retornado por `Next()`. Overhead: uma observação de histograma e dois counters (`WithLabelValues`, sem alocação de mapa extra); desprezível frente ao SQLite. Benchmark simples na tarefa para registrar o custo.

### 10.5 Listener dedicado

**Endereço e política de subida — DECIDIDO (fail-closed):** o listener **só sobe quando `metrics.addr` está configurado**; sem config, nenhum socket extra é aberto (comportamento atual preservado, E2E existentes intactos). Quando configurado, o valor sugerido é **`127.0.0.1:9091`** em host/systemd (9091 e não 9090, para não colidir com o próprio Prometheus no mesmo host) e **`0.0.0.0:9091`** apenas dentro do container (`deploy/config.docker.yaml`), onde a rede do compose é a barreira.

**Config.** `internal/config`: `MetricsConfig{ Addr string }`; `yamlDoc.Metrics.Addr` (`yaml:"addr"`); `Config.Metrics`; `EnvMetricsAddr = "PSICOMAN_METRICS_ADDR"`; `resolve("metrics.addr", EnvMetricsAddr, doc.Metrics.Addr)` com precedência env > arquivo (a env facilita Docker e o E2E, que não monta YAML). Validação em `Load`: se não vazio, `net.SplitHostPort` deve funcionar e a porta ser 1..65535; senão erro de config (boot aborta com mensagem clara). Teste unit em `internal/config` (válido, vazio, inválido, env sobrescreve arquivo).

**Servidor (`internal/api/metricsserver.go`).** Dentro do `OnServe`, depois de montar as rotas e antes de `e.Next()`:
- `http.Server{Addr, Handler: mux, ReadHeaderTimeout: 5s, ReadTimeout: 10s, WriteTimeout: 15s}` com um `http.ServeMux` próprio contendo **só** `GET /metrics` → `promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError, Timeout: 10 * time.Second, MaxRequestsInFlight: 4})`. Todo o resto responde 404 do mux padrão. Sem envelope JSON (formato de exposição do Prometheus), sem gate do Pangolin.
- `net.Listen` primeiro (síncrono): falha de bind (porta em uso) **aborta o boot** com erro explícito, já que a config foi pedida de forma deliberada e um listener silenciosamente ausente esconde a falta de observabilidade. Depois `go srv.Serve(ln)`.
- Encerramento: `e.App.OnTerminate().BindFunc` chamando `srv.Shutdown(ctx 5s)`; `e.Next()` no hook (mesmo padrão do `pbGracefulShutdown` do PB).
- Log de boot: `Info` com o endereço; `Warn` adicional se o host for `0.0.0.0`/`::`/vazio ("listener de métricas em todas as interfaces: garanta rede fechada").
- O listener de métricas **não** é instrumentado por si mesmo (evita ruído de scrape nas séries).

**A API não serve métricas (R8.9.3).** Nenhuma rota `/metrics` nem `/v1/metrics` é registrada no PocketBase; ambas caem no catch-all e respondem **404** (confirmado pelo comportamento do `BuildMux`; o E2E prova). **VERIFICADO no mux do PB (2026-11-05, spike do `Pattern`):** `GET /metrics` na API responde 404 com `Pattern == "/"` (catch-all). O E2E contra o binário real (com o Dashboard registrado) continua obrigatório e reconfirma.

**Conflito com ADR.** O ADR de 2026-10-24 ("Segregação de exposição na borda, Opção A") descartou dois listeners no mesmo processo **para segregar atores**. O listener de métricas não segrega atores e não serve a API: é uma superfície interna de observabilidade. Novo ADR em `docs/decisions.md` (a) encerra o adiamento de métricas, (b) referencia e reconcilia o ADR de 2026-10-24, (c) registra `client_golang` v1.25.0, listener dedicado, fail-closed, e os rótulos proibidos. `docs/architecture.md §0.21` é ajustado.

### 10.6 Política de rede recomendada (Recomendação de design)

- O endpoint de métricas **nunca** é publicado na borda: não entra no `portal.conf` (que já responde 404 a tudo fora de `/v1/portal/*` e das sondas), não entra no Pangolin, não tem `ports:` no compose. No `docker-compose.yml` e no `Dockerfile`: `expose: "9091"`/`EXPOSE 9091` apenas documentais; `deploy/config.docker.yaml` ganha `metrics.addr: "0.0.0.0:9091"` com comentário.
- Ambiente de coleta: o Prometheus (ou agente) roda na **mesma rede interna/VLAN** do container (rede Docker dedicada de observabilidade ou host-only) e acessa `psicoman:9091` por nome. Regra de firewall/security group: permitir TCP 9091 **somente** do IP do coletor; negar o resto. Sem TLS nem autenticação no listener nesta onda, porque o isolamento é de rede; se a topologia mudar (coleta atravessando rede não confiável), o passo seguinte é TLS mútuo no listener ou um reverse proxy com mTLS (fora de escopo, registrado no ADR).
- Em host sem container: `metrics.addr: "127.0.0.1:9091"` e coleta por agente local (ou túnel SSH).
- Alertas sugeridos para `docs/deploy.md`: `time() - psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="drive"} > 2*86400`; `psicoman_google_oauth_ok == 0`; `psicoman_identidade_dev_ativa == 1` em produção; `rate(psicoman_http_requisicoes_total{status=~"5.."}[5m]) > 0`; `psicoman_drive_snapshots_mantidos > N+3`; `increase(psicoman_audit_log_falhas_total[1h]) > 0`; `increase(psicoman_cifra_operacoes_total{resultado="erro"}[5m]) > 0`; `increase(psicoman_rate_limit_rejeicoes_total[10m]) > 100` (pico de rejeições, possível saturação do limitador).

### 10.7 Dependência

`go get github.com/prometheus/client_golang@v1.25.0` + `go mod tidy` (versão **exata** fixada; transitivas `client_model`, `common`, `procfs`, `protobuf`, `beorn7/perks`, `cespare/xxhash`). **VERIFICADO (2026-11-05):** `go list -m -versions github.com/prometheus/client_golang` lista `v1.25.0`; um módulo temporário em `/tmp` com `go 1.27` importando `prometheus`, `collectors` e `promhttp` passa `go build` e `go vet` (go1.27.1) e executa. Simulação do `go mod tidy` numa **cópia** do repositório (sem `.git`, com um arquivo temporário que importa os três pacotes): `go get ...@v1.25.0` + `go mod tidy` + `go build ./...` saem 0, e o diff do `go.mod` é **só de acréscimos** (`client_golang v1.25.0` direta; indiretas `beorn7/perks v1.0.1`, `cespare/xxhash/v2 v2.3.0`, `munnerz/goautoneg`, `prometheus/client_model v0.6.3`, `prometheus/common v0.72.0`, `prometheus/procfs v0.22.0`, `google.golang.org/protobuf v1.36.12`): **nenhuma versão existente, inclusive as do PocketBase, foi alterada**. Plano original mantido: primeira tarefa de R8 reconfirma no repositório real: rodar, conferir `go build ./...`, `go test ./...`, e o diff do `go.sum`; se houver conflito, parar e reportar. Rodar `govulncheck` se disponível (não instalado nesta onda; registrar).

### 10.8 Testes de R8

- **Unit `httpx`:** `RouteLabels` (tabela: padrões de cada ator, `/` → `nao_encontrada`, vazio → `desconhecida`, nunca ULID); middleware com `HTTPRecorder` fake e `core.RequestEvent` de teste: contagem, status via `err` (404), `denyReason`, recorder que panica não altera a resposta (R8.10).
- **Unit `platform/metrics`:** cada família aparece com o rótulo esperado; `NopMetrics` não panica; Collector de filas com fonte que erra omite a amostra e conta o erro; cache de 15 s (relógio fake).
- **Unit serviços:** cada serviço com `Metrics` fake (contador em memória) conta os eventos certos nos caminhos ok/erro (paciente, sessão, pedido, débito, pagamento, PDF, auditoria).
- **archtest:** `TestDomainAppConfigNaoImportamPrometheus`.
- **E2E (`test/e2e/metrics_test.go`)**, binário real com `PSICOMAN_METRICS_ADDR` em `freePort` extra (helper `startServerWithMetrics` devolve `base` e `metricsBase`):
  1. `GET <metricsBase>/metrics` → 200, `Content-Type` texto Prometheus, contém `go_goroutines` e `psicoman_build_info`.
  2. Gera tráfego real: `GET /v1/livez` (200), `GET /v1/app/patients` sem header (401), `GET /v1/portal/sessions` sem sessão (401), rota inexistente `GET /v1/nao-existe` (404), `GET /v1/app/patients/<ulid>` com header (404 de paciente) — e confere `psicoman_http_requisicoes_total` com `ator`, `rota="/v1/app/patients/{id}"`, `status` corretos, `seguranca_negados_total{motivo="sem_identidade"}` e `rota="nao_encontrada"`.
  3. **Caso 403** do portal: paciente cadastrado e **não aprovado** com sessão real, `GET /v1/portal/appointment-requests` → 403; confere `motivo="nao_aprovado"`. Permitido e negado juntos (DoD).
  4. **Vazamento:** cadastra paciente com e-mail/CPF/nome/telefone sentinela, adiciona nota clínica com texto sentinela, usa a rota `/v1/app/patients/<ulid real>`; depois captura `/metrics` inteiro e afirma que **nenhum** sentinela, o ULID, o valor do header `X-Pangolin-User`, nem fragmento da chave de cifra aparece. Também varre os valores de todos os rótulos contra a lista de enums permitidos para `ator`, `metodo`, `status`.
  5. **Isolamento:** `GET <base>/metrics` → 404 e `GET <base>/v1/metrics` → 404 na porta da API; `GET <metricsBase>/v1/livez` → 404 no listener de métricas.
  6. **Fail-closed:** servidor sem `metrics.addr` não abre porta (conexão recusada no endereço que seria usado, o teste **reserva** uma porta com `net.Listen("tcp", "127.0.0.1:0")`, anota o endereço e a **fecha**, sobe o servidor **sem** `PSICOMAN_METRICS_ADDR` e afirma que `net.Dial` nesse endereço é recusado. Limitação aceita e comentada no teste: outro processo poderia reutilizar a porta no intervalo (flake improvável, sem tentar reabrir a porta); a garantia primária é o unit de `config`/`metricsserver` que afirma que, com `Addr == ""`, nenhum `net.Listen` é chamado (listener injetável por função).
  7. Backup: `POST /v1/admin/backup` e conferência de `backup_total{destino="local",resultado="ok"}` e do timestamp > 0.
- **Em processo** (`internal/api`): gauge `identidade_dev_ativa` com seam `configLoader` devolvendo `DevIdentity`; `google_oauth_ok` coerente com `/readyz`; métricas de Calendar/Gemini/Drive com os fakes reatribuídos nos seams `calendarWirer`, `aiSummarizerBuilder`, `driveSyncWirer`.

**Docs/Swagger.** `/metrics` fica fora de `/v1` e do contrato Swagger (não é rota da API). `docs/deploy.md` (nomes das métricas, como coletar, alertas, política de rede), `docs/architecture.md`, `docs/decisions.md`, `docs/stack-local.md`, `config.example.yaml`, `deploy/config.docker.yaml`, `docker-compose.yml`, `Dockerfile`.

### 10.9 Assinaturas Go de R8

Fecha a superfície referenciada em §10.2 a §10.8. Nomes finais; o implementador não decide arquitetura, só preenche corpos.

```go
// internal/app/metrics.go  (complemento de §10.2; sem imports de PocketBase/Prometheus)
type CalendarOp string // CalendarFreeBusy = "freebusy" | CalendarCreateEvent = "create_event"
type CipherOp string   // CipherSeal = "seal" | CipherOpen = "open"
type QueueStats interface {
    PendingPatients(ctx context.Context) (int, error)
    PendingAppointments(ctx context.Context) (int, error)
}

// internal/api/httpx/metrics.go  (não importa client_golang)
type HTTPRecorder interface {
    Request(ator, rota, metodo string, status int, d time.Duration, bytes int64)
    InFlight(ator string, delta int)           // +1 / -1
    Denied(ator string, status int, motivo string)
    RateLimited(scope string)                  // chamado por Limiter.OnReject (R7.1)
}
type DenyReasonCode string // conjunto fechado:
const (
    DenySemIdentidade DenyReasonCode = "sem_identidade"
    DenySemSessao     DenyReasonCode = "sem_sessao"
    DenyNaoAprovado   DenyReasonCode = "nao_aprovado"
    DenyPocketBase    DenyReasonCode = "pocketbase"
    DenyOutro         DenyReasonCode = "outro"
)
func SetDenyReason(e *core.RequestEvent, r DenyReasonCode) // guarda em e.Set
func DenyReason(e *core.RequestEvent) DenyReasonCode        // "" se ausente
func RouteLabels(pattern string) (ator, rota string)        // §10.4
// Instrument devolve o handler do middleware raiz (registrado com Priority -5000 em api.go).
func Instrument(rec HTTPRecorder) func(*core.RequestEvent) error

// internal/platform/metrics  (única camada, com api/metricsserver.go, que importa client_golang)
type BuildInfo struct{ Version, GoVersion string }
type Config struct {
    Build       BuildInfo
    Queues      app.QueueStats // nil = omite as gauges de fila
    DevIdentity bool           // alimenta identidade_dev_ativa
    AIEnabled   bool           // alimenta gemini_ligada
    GoogleOK    func() bool    // fonte de google_oauth_ok (mesmo critério do /readyz); nil = omite
    Clock       clock.Clock    // cache de 15 s do Collector e uptime; nil = relógio real
}
type Obs struct{ /* registry *prometheus.Registry; coletores */ }
func New(c Config) *Obs
func (o *Obs) Handler() http.Handler          // promhttp.HandlerFor(registry, ...) (opções de §10.5)
func (o *Obs) App() app.Metrics               // porta de negócio (§10.2)
func (o *Obs) HTTP() httpx.HTTPRecorder       // recorder do middleware e do limitador
func (o *Obs) Drive() drivesync.Observer      // adapter de §3.1 (deriva drive_dr_pronto e o timestamp)
// Decoradores (instrumentação sem tocar os serviços):
func WrapCipher(c domain.Cipher, m app.Metrics) domain.Cipher
func WrapAI(s app.AISummarizer, m app.Metrics) app.AISummarizer
// VERIFICADO (2026-11-05): não existe app.CalendarGateway; as interfaces reais são duas (internal/app/ports.go).
func WrapConflictChecker(c app.ConflictChecker, m app.Metrics) app.ConflictChecker // op "freebusy"  (HasConflict)
func WrapSessionCalendar(c app.SessionCalendar, m app.Metrics) app.SessionCalendar // op "create_event" (CreateEvent)

// internal/api/service (wiring)
// Services.Obs *metrics.Obs: criado em buildServices por metrics.New(...) quando metrics.addr != ""
// OU sempre (registry barato) e só o listener depende do addr. DECISÃO: criado SEMPRE, para os
// serviços usarem o mesmo caminho de código com e sem listener; Obs.App() alimenta SetMetrics dos serviços.
// Sem listener, o registry existe e ninguém o lê (custo desprezível).
// Services.DriveStatus *drivesync.Status (§3.1).

// internal/api/metricsserver.go
var listen = net.Listen // seam de teste (substituível por função que registra a chamada)
// StartMetricsServer sobe o listener dedicado (§10.5). addr == "" retorna nil sem chamar listen (fail-closed).
// Registra Shutdown em app.OnTerminate().
func StartMetricsServer(app core.App, addr string, h http.Handler, log *slog.Logger) error
```

`QueueStats` é injetado em `metrics.Config.Queues` por `buildServices` (implementação `pb.NewQueueStats(a)`); o `Collector` custom do `Obs` o consulta com o timeout e o cache de §10.2. `DevIdentity` vem de `cfg.Pangolin.DevIdentity != ""` e `AIEnabled` de `cfg.Gemini.Enabled()` (**VERIFICADO em `internal/config/config.go`:** `Config.Pangolin.DevIdentity string` e `Config.Gemini GeminiConfig` com o método `Enabled()` que faz `TrimSpace(APIKey) != ""`; **não** existe `cfg.LLM.Gemini`, esse é só o caminho do YAML).

---

## 11. Seções transversais

### 11.1 Ordem de dependência

```
R4 (lint)  ──► (cedo, para acompanhar o código novo)
R7.4 ator ctx ─► R1 ─► R5          (R1 usa o ator do contexto; R5 testa)
R1 ───────────► R8.7 (audit_log por ação)
R3 ───────────► R8.6 (pasta/DR/snapshots)  ;  R2 retenção ─► R8.6 (snapshots mantidos)
R8 base (dep + porta + registry + middleware + listener) ─► R8 famílias de negócio/integração
R6 (docs) acompanha cada requisito de origem; fecha no final
```

Sugestão de fases (um commit por fase, `make check` verde antes; branch `auditoria-profunda`):
1. **F0 R4:** spike do linter; migração do YAML, Makefile, CI; triagem em commits separados. *Fazer primeiro*: o código novo das fases seguintes já nasce sob o gate. Atenção: achados em massa; se passarem de ~100, usar `new-from-rev` e abrir a tarefa de dívida.
2. **F1 R7.4 + R1 + R5:** `domain/actor`, middleware, `AuditService`, `ProntuarioService.SetAudit`, testes. ADR de auditoria.
3. **F2 R2 (doc) + R3:** ADR snapshot inteiro, steering do Drive, migration `0050`, `DriveClient` (folder/list/trash), `FolderResolver`, retenção, `/readyz`. Verificação manual de `drive.file` (list/trash) com credencial de dev.
4. **F3 R7.1 e R7.2:** limitador e store de `state`; atualização dos testes que quebram.
5. **F4 R8 base:** spike (`Pattern`, `countingWriter`, `go mod tidy`), porta `app.Metrics`, `platform/metrics`, middleware raiz, config, listener, archtest, E2E de isolamento/HTTP/vazamento.
6. **F5 R8 famílias:** negócio, segurança, Calendar/Gemini/cifra/OAuth, auditoria (depende de F1), backup/Drive (depende de F2), filas.
7. **F6 R6 + R7.3 + fechamento:** steerings (Gmail), `architecture.md`/`deploy.md`/`stack-local.md`/`requirements.md`, tabela da Fase 10, checklist de rotação, `release-notes.md`, push **somente** com OK do Rafael.

### 11.2 Riscos e mitigação

| Risco | Efeito | Mitigação |
|-------|--------|-----------|
| golangci-lint v2.15.0 também não decodifica o export data do Go 1.27 | R4 bloqueada | Spike no início da F0; parar e levar ao Rafael; nunca reintroduzir skip silencioso |
| Achados em massa do linter | diff gigante, fases contaminadas | Commits separados; baseline `new-from-rev`; dívida em tarefa à parte |
| Retenção apaga snapshot errado | perda de ponto de DR | Lixeira (30 dias), 4 travas (prop, `.enc`, pasta, recém-enviado), `N=14`, teste de tabela, `0` desliga |
| `drive.file` não permite `list`/`update` | retenção e busca de pasta indisponíveis | Verificação manual cedo na F2; fallback: retenção manual no ADR, busca de pasta cai para criação (e duplicata é detectável pelo operador) |
| Pasta de DR duplicada | confusão no Drive | Ordem config > persistido > busca por `appProperties` > criação; mutex; teste de reuso |
| `Request.Pattern` vazio ou `countingWriter` quebrando `StatusTracker`/SSE | rótulo ruim ou regressão no realtime | Spike com teste; plano B (sem bytes); `rota="desconhecida"` em vez de path bruto |
| Cardinalidade explosiva | memória no coletor | Rota = padrão; enums fechados; revisão de rótulos no E2E de vazamento |
| `go mod tidy` com `client_golang` mexe em versões do PB | build quebra | Spike da dependência na F4; parar se houver conflito |
| Cookie de `state` não chega atrás do Pangolin | OAuth quebra em prod | `SameSite=Lax` + `Path` fixo; teste manual no deploy; mensagem clara; fluxo continua refazível |
| Tabela do limitador saturada por botnet (`maxLimiterKeys = 10000`) | cadastros novos negados por até 1 h | Exige >5 IPs sustentados (nginx 30 r/min/IP); `Warn` limitado + métrica; chaves existentes seguem atendidas; risco aceito no ADR |
| Rate limit em memória zera no restart / só vale por processo | janela de abuso após restart | Aceito (single-tenant, processo único); nginx por IP continua; documentado |
| Audit best-effort perde registro | lacuna de trilha | Log `Error` + métrica `audit_log_falhas_total` + alerta; ADR registra a escolha |
| Enumeração de e-mail no `register` (409) | vazamento de existência | Rate limit reduz a taxa; risco aceito e registrado; backlog para resposta uniforme |
| Listener de métricas exposto por engano (`0.0.0.0` no host) | métricas acessíveis fora da rede | Fail-closed por padrão, `Warn` no boot para bind amplo, política de firewall documentada, E2E de isolamento da API |
| Segredos reais em `config.dev.yaml` | exposição já ocorrida | R7.3: rotacionar ao fim dos testes; não ler/copiar esse arquivo em testes |

### 11.3 Rollout e compatibilidade

Tudo é aditivo e com defaults seguros:
- **Config nova, toda opcional:** `metrics.addr` (ausente = sem listener), `google.drive_retention` (ausente = 14), `google.drive_folder` (agora opcional). Instâncias existentes sobem sem alterar YAML; elas passam a **criar a pasta e fazer a retenção** no próximo backup se já tinham credenciais e refresh token (e, se já usavam `drive_folder`, nada muda). Uma instância que tinha DR **desativado** por falta de `drive_folder` passa a ativá-lo (efeito desejado de R3.4); a nota de migração no release avisa.
- **Migration `0050`** é de criação de coleção (sem alteração de dados); roda no boot.
- **Contrato `/v1`:** só acréscimos (`429 limite_excedido`, `400 state_invalido`, `checks.drive_dr` no `/readyz`). O callback do OAuth exige o cookie do authorize (quem tinha o fluxo aberto antes do deploy precisa reiniciá-lo).
- **Snapshots antigos** no Drive (sem `appProperties`) ficam fora da retenção.
- **Rollback:** reverter o binário; a coleção `drive_dr` fica órfã e inofensiva.
- **Commit por fase; push só no fim e com OK do Rafael**, conforme `psicoman-processo-entrega.md`.

### 11.4 Fechamento de fase e DoD

Cada fase fecha com o checklist de `psicoman-processo-entrega.md`: `make check` verde (já com lint real após F0), testes de unit e E2E afetados, `docs/requirements.md`/`architecture.md`/`README.md`/`deploy.md`, `docs/decisions.md` (ADRs desta onda: auditoria clínica; snapshot inteiro e retenção; pasta de DR e persistência; linter v2; state do OAuth e rate limit; métricas e listener, referenciando o ADR de 2026-10-24), Swagger quando o contrato mudar, e `docs/release-notes.md` no fechamento da onda. Cada spec gerada por auditoria tem entrada correspondente em `docs/decisions.md` apontando para `.kiro/specs/auditoria-profunda/`.

### 11.5 Estado das verificações (resolvidas em 2026-11-05)

| # | Suposição original | Resultado | Evidência / o que fica |
|---|--------------------|-----------|------------------------|
| 1 | golangci-lint v2.15.0 (go1.27.1) decodifica o export data | **VERIFICADO** | `go install` em `/tmp/gobin`, `run` com config migrada: 185 achados (195 com o rascunho do §4); sem erro de export data. Tarefa 1.1 só reconfirma |
| 2a | `drive.file` permite `files.list`/`files.update`/`files.create` de pasta sobre arquivos do app | **VERIFICADO PELA DOCUMENTAÇÃO OFICIAL**; runtime pendente | Doc do Google sobre `drive.file`; spike manual 4.1 com credencial de dev confirma em runtime |
| 2b | Reação do Drive a upload com pai na lixeira ou apagado | **NÃO VERIFICÁVEL LOCALMENTE** | Exige credencial real; o design é robusto a qualquer resultado (revalida `trashed` no id persistido); spike manual na tarefa 4.1 |
| 2c | Upload com pai criado à mão sob `drive.file` | **VERIFICADO empiricamente** (conta real do Rafael) | Upload funcionou; a busca só vê o que o app criou (ver §3) |
| 3a | `Request.Pattern` preenchido no mux do PB | **VERIFICADO em runtime** | Tabela do §10.4: inclui prefixo de método, catch-all `/`, já populado antes dos middlewares do grupo |
| 3b | `countingWriter` compatível com `StatusTracker`/SSE | **VERIFICADO por leitura de código**; runtime na tarefa 7.1 | `getStatus`/`getWritten` seguem `Unwrap()` |
| 4 | `go mod tidy` com `client_golang@v1.25.0` sem conflito | **VERIFICADO** | Cópia do repositório: só acréscimos no `go.mod`; build ok |
| 5 | `/metrics` na porta da API responde 404 | **VERIFICADO no mux**; E2E no binário real continua obrigatório | Spike do `Pattern` |
| 6a | Nome do fake de `AuditRepository` | **VERIFICADO** | `fakeAuditRepo` em `internal/app/fakes_test.go` |
| 6b | Ramo "agenda sem calendar" | **VERIFICADO** | `createCalendarEvent`, `session_service.go:293`; interfaces `SessionCalendar`/`ConflictChecker` (**REFUTADO** o nome `CalendarGateway`) |
| 6c | `contract_test.go` compara respostas? | **VERIFICADO: não** | Só rotas + `responses` não vazio; declarar `429` em `openapi.base.json` |
| 7 | Merge do PR #4 | **VERIFICADO** | `5aeeabe Merge pull request #4 from RafaelFino/pocketbase-finish` |
| 8 | Método de atributos/tamanho do `filesystem.System` | **VERIFICADO** | `Attributes(fileKey) (*blob.Attributes, error)`, `.Size int64` |
| 9 | Nomes dos campos de config (`DevIdentity`, Gemini) | **VERIFICADO** | `cfg.Pangolin.DevIdentity`, `cfg.Gemini.Enabled()` |

Única suposição que continua aberta: **2b** (e a confirmação em runtime de **2a**), ambas dependem de credencial real do Google e têm spike manual na tarefa 4.1.

---

## 12. Matriz de cobertura (requisito → seção)

| Requisito | Seção do design |
|-----------|-----------------|
| R1.1, R1.2, R1.3, R1.4 | [§1](#1-r1--auditoria-lgpd-no-caminho-clínico), [§6](#6-r74--ator-do-audit-pela-identidade-real) |
| R2.1, R2.2, R3.2 | Descartados por decisão do Rafael; nada a implementar |
| R2.3 | [§2](#2-r2--snapshot-inteiro-oficializado-e-retenção), [§9 R6.1](#9-r6-e-r73--documentação-steering-e-operação) |
| R2.4 | [§2](#2-r2--snapshot-inteiro-oficializado-e-retenção) (retenção N=14, lixeira) |
| R3.1, R3.4 | [§3](#3-r3--pasta-de-dr-criada-automaticamente) |
| R3.3 | [§3](#3-r3--pasta-de-dr-criada-automaticamente) (log, métrica, `/readyz`) |
| R4.1, R4.2 | [§4](#4-r4--golangci-lint-reativado) |
| R5.1, R5.2 | [§1](#1-r1--auditoria-lgpd-no-caminho-clínico), [§5](#5-r5--testes-da-auditoria-clínica) |
| R6.1, R6.2, R6.3 | [§9](#9-r6-e-r73--documentação-steering-e-operação) |
| R7.1 | [§7](#7-r71--rate-limit-por-e-mail) |
| R7.2 | [§8](#8-r72--validação-do-state-do-oauth) |
| R7.3 | [§9](#9-r6-e-r73--documentação-steering-e-operação) |
| R7.4 | [§6](#6-r74--ator-do-audit-pela-identidade-real) |
| R8.1 | [§10.5](#105-listener-dedicado) |
| R8.2 | [§10.3](#103-famílias-de-métricas-prefixo-psicoman_), [§10.4](#104-middleware-http-e-mapeamento-da-rota-padrão) |
| R8.3 | [§10.3](#103-famílias-de-métricas-prefixo-psicoman_), [§10.4](#104-middleware-http-e-mapeamento-da-rota-padrão) (motivo de 401/403, gauge dev) |
| R8.4 | [§10.2](#102-porta-de-negócio), [§10.3](#103-famílias-de-métricas-prefixo-psicoman_) |
| R8.5 | [§10.2](#102-porta-de-negócio) (decorador do Calendar), [§10.3](#103-famílias-de-métricas-prefixo-psicoman_) |
| R8.6 | [§10.3](#103-famílias-de-métricas-prefixo-psicoman_), [§2](#2-r2--snapshot-inteiro-oficializado-e-retenção), [§3](#3-r3--pasta-de-dr-criada-automaticamente) |
| R8.7 | [§10.2](#102-porta-de-negócio), [§10.3](#103-famílias-de-métricas-prefixo-psicoman_), [§1](#1-r1--auditoria-lgpd-no-caminho-clínico) |
| R8.8 | [§10.1](#101-decisões-de-arquitetura), [§10.3](#103-famílias-de-métricas-prefixo-psicoman_) (decorador de cifra, runtime, process) |
| R8.9.1, R8.9.2, R8.9.4 | [§10.3](#103-famílias-de-métricas-prefixo-psicoman_), [§10.4](#104-middleware-http-e-mapeamento-da-rota-padrão), [§10.8](#108-testes-de-r8) (teste de vazamento) |
| R8.9.3 | [§10.5](#105-listener-dedicado), [§10.6](#106-política-de-rede-recomendada-recomendação-de-design), [§10.8](#108-testes-de-r8) |
| R8.10 | [§10.4](#104-middleware-http-e-mapeamento-da-rota-padrão) |
| R8.11 | [§10.1](#101-decisões-de-arquitetura), [§10.7](#107-dependência) |
| R8 (assinaturas Go) | [§10.9](#109-assinaturas-go-de-r8) |
| R9 (rastreabilidade) | Nada a implementar. Os itens FEAT-1, OAUTH-1, OBS-1 (probes) e ARQ-1 não são reabertos; o design só **estende** o `/readyz` com `checks.drive_dr` (R3.3), sem alterar o comportamento existente |
| R10 (DoD) | [§11.4](#114-fechamento-de-fase-e-dod) |

---

## 13. Respostas à revisão do design

| # | Severidade | Achado | Resposta |
|---|-----------|--------|----------|
| 1 | MEDIUM | Contrato interno do `drivesync` incompleto | **Endereçado** em [§3.1](#31-contrato-interno-do-drivesync): `SnapshotStore`, `File`, `Observer`, `Status`, `New(store, sealer, retention, obs, st)`, medição do backup local no hook, origem de `drive_snapshots_mantidos`, `Services.DriveStatus` para `checks.drive_dr`, seam `driveSyncWirer` |
| 2 | MEDIUM | `maxKeys` do limitador sem valor | **Endereçado** em [§7](#7-r71--rate-limit-por-e-mail): `maxLimiterKeys = 10000`, justificativa numérica, testes de saturação, `Warn` limitado, risco aceito no ADR, alerta |
| 3 | NIT | Logger e métrica do `AuditService` | **Endereçado** em [§1](#1-r1--auditoria-lgpd-no-caminho-clínico): `SetLogger(*slog.Logger)` (default `slog.Default()`) e `SetMetrics(Metrics)`; wiring injeta `a.Logger()` |
| 4 | NIT | E2E fail-closed frágil | **Endereçado** em [§10.8](#108-testes-de-r8) item 6: reserva/fecha a porta, sobe sem `metrics.addr`, `Dial` recusado; limitação documentada; garantia primária por unit com listener injetável |
| 5 | NIT | `Secure` depende de `X-Forwarded-Proto` | **Endereçado** em [§8](#8-r72--validação-do-state-do-oauth): é só endurecimento; `docs/deploy.md` documenta o repasse do header |
| 6 | NIT | Frase sobre `requirements.md` não alterado | **Endereçado** no cabeçalho e em `requirements-notes.md`: "não reescrito por esta etapa"; origem do diff a confirmar antes do commit |

---

### Segunda rodada de revisão

| # | Severidade | Achado | Resposta |
|---|-----------|--------|----------|
| 1 | MEDIUM | Métrica e log de rejeição do rate limit sem caminho até o recorder | **Endereçado** em [§7](#7-r71--rate-limit-por-e-mail): `LimiterOptions{Scope, Log, OnReject, Clock}`, `RateLimited(scope)` no `HTTPRecorder` ([§10.9](#109-assinaturas-go-de-r8)), `portal.Mount` liga `OnReject`; testes cobrem estouro, saturação e `ok` |
| 2 | MEDIUM | Assinaturas Go de R8 incompletas | **Endereçado** em [§10.9](#109-assinaturas-go-de-r8): `HTTPRecorder`, `DenyReasonCode`/`SetDenyReason`/`DenyReason`, `CalendarOp`, `CipherOp`, `QueueStats`, `platform/metrics` (`Config`, `Obs`, decoradores), `Services.Obs`, `StartMetricsServer` com seam `listen` |
| 3 | MEDIUM | 404 do Drive para pasta apagada/lixeira assumido como fato | **Endereçado** em [§3](#3-r3--pasta-de-dr-criada-automaticamente): rotulado como SUPOSIÇÃO NÃO VERIFICADA, spike manual, revalidação `files.get?fields=id,trashed` na primeira resolução por processo, unit "persistido na lixeira" |
| 4 | NIT | `Invalidate` sem como apagar o id persistido | **Endereçado** em [§3](#3-r3--pasta-de-dr-criada-automaticamente): resolução pula o passo 2 e o `Save` sobrescreve o singleton; sem `Delete` |
| 5 | NIT | Rótulos divergentes `register/appt` vs `cadastro/agendamento` | **Endereçado** em [§7](#7-r71--rate-limit-por-e-mail): enum único `ScopeCadastro`/`ScopeAgendamento` |
| 6 | NIT | `Observer` sem `drive_dr_pronto`/timestamp | **Endereçado** em [§3.1](#31-contrato-interno-do-drivesync): nota de derivação no adapter; sem métodos novos |

---

## Respostas à revisão (rodada 4)

| # | Severidade | Achado | Resposta |
|---|-----------|--------|----------|
| 1 | MEDIUM | `ErrDriveNotFound` inalcançável pelo `drivesync` | **Endereçado** em [§3](#3-r3--pasta-de-dr-criada-automaticamente) e §3.1: `drivesync.ErrFolderNotFound` e `ErrNoStoredFolder` em `folder.go`; adapter traduz `google.ErrDriveNotFound` (e `app.ErrNotFound` no `Load`). Fato corrigido: `google.ErrDriveNotFound` ainda não existe no código, é novo desta onda |
| 2 | MEDIUM | `Status.State()` sem `desativado`/`ok sem tentativa` | **Endereçado** em §3.1: `SetConfigured`, chamado por `New` com `store != nil`; tabela de `State()`, valor zero `desativado`, teste unitário |
| 3 | NIT | Prioridade `-5000` impossível com `BindFunc` | **Endereçado** em [§10.4](#104-middleware-http-e-mapeamento-da-rota-padrão): `e.Router.Bind(&hook.Handler[*core.RequestEvent]{...Priority: -5000})` com import de `tools/hook` |
| 4 | NIT | Ordem do rate limit vs `senha_fraca` | **Endereçado** em [§7](#7-r71--rate-limit-por-e-mail): após `BindBody`, antes da validação de senha (confirmado em `portal/me.go`) |
| 5 | NIT | Onde mora o filtro `.enc` | **Endereçado** em [§2](#2-r2--snapshot-inteiro-oficializado-e-retenção) (mecânica, item 4): props/pasta no `List` do adapter, `HasSuffix(".enc")` em `selectToTrash` |
| 6 | NIT | Matriz sem R2.1, R2.2, R3.2 | **Endereçado** em [§12](#12-matriz-de-cobertura-requisito--seção): nova linha |

---

## Decisões fechadas (2026-11-05, por delegação do Rafael; ajustáveis na revisão)

O Rafael pediu para "resolver" os pontos em aberto. Ficam **DECIDIDOS** (não são mais propostas):

1. **Retenção de 14 snapshots do Drive, por lixeira** (`trashed`, não `delete`) (§2).
2. **Listener de métricas só sobe quando `metrics.addr` está configurado**: `127.0.0.1:9091` no host e `0.0.0.0:9091` no container, porta **não publicada** (§10.5).
3. **Auditoria best-effort:** falha ao gravar `audit_log` loga (`Error`) e conta a métrica, mas **não derruba a requisição** (§1).
4. **Rate limit:** 5 cadastros por hora por e-mail normalizado e 10 pedidos de agendamento por hora por paciente (§7).
5. **Baseline do linter** (`issues.new-from-rev`) se a triagem passar de ~100 achados (§4). Número real medido: 185 achados (195 com o rascunho); a tarefa 1.5 aplica a regra **depois** de desligar, com justificativa, os três ruídos medidos (`misspell` PT-BR, `revive:error-strings`, `revive:unused-parameter`), o que deixa ≈35.
6. **Achados fora de escopo para backlog:** enumeração por `409` no `register`; login nativo do paciente sem limite próprio e bloqueado em `/api/` na borda pública (§7).
