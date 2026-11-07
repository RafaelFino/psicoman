package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/drivesync"
	"github.com/RafaelFino/psicoman/internal/integration/gemini"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/metrics"
	"github.com/RafaelFino/psicoman/internal/platform/pb"
)

// calendarReminderMinutes é o default de lembretes do evento: 1 dia + 30 min
// (acumulativos) — psicoman-google-api.md.
var calendarReminderMinutes = []int{1440, 30}

// EnvGoogleRefreshToken carrega o refresh token OAuth do terapeuta. O fluxo
// 3-legged + persistência cifrada do token (coleção google_token) chega numa
// fase posterior; por ora o refresh token vem do ambiente para ativar o
// Calendar real quando as credenciais do app também estão presentes.
const EnvGoogleRefreshToken = "PSICOMAN_GOOGLE_REFRESH_TOKEN"

// cipherConfig é a configuração de cifra resolvida no boot.
type cipherConfig struct {
	CurrentKeyID string
	Keys         []crypto.KeySpec
}

// configLoader é a fonte única de configuração do boot (internal/config). Seam
// análogo a cipherConfigLoader: reatribuível para testes que queiram injetar um
// Config determinístico sem tocar o ambiente global.
var configLoader = func() (config.Config, []config.OriginEvent, error) {
	return config.Load(config.Options{})
}

// cipherConfigLoader resolve a config de cifra. Default deriva de configLoader()
// (internal/config); os testes em processo trocam este seam por um loader
// determinístico com a KeySpec fixa, sem depender de env vars globais (§7).
var cipherConfigLoader = loadCipherConfig

// loadCipherConfig deriva a config de cifra do Config carregado por
// configLoader() (arquivo YAML + env sobrescreve). Cifra ausente/incoerente →
// erro, abortando o boot (fail-closed). O base64 e o tamanho (32 bytes) são
// validados depois por NewMapKeyProvider.
func loadCipherConfig() (cipherConfig, error) {
	cfg, _, err := configLoader()
	if err != nil {
		return cipherConfig{}, err
	}
	keyID := cfg.Crypto.KeyID
	if keyID == "" || len(cfg.Crypto.Keys) == 0 {
		return cipherConfig{}, fmt.Errorf("config de cifra ausente: defina %s e %s (ou crypto.key_id/crypto.keys no arquivo)", config.EnvCipherKeyID, config.EnvCipherKeys)
	}
	specs := make([]crypto.KeySpec, 0, len(cfg.Crypto.Keys))
	for id, b64 := range cfg.Crypto.Keys {
		specs = append(specs, crypto.KeySpec{ID: id, B64Key: b64})
	}
	return cipherConfig{CurrentKeyID: keyID, Keys: specs}, nil
}

// buildServices monta repos + cipher + serviços + hooks de sessão. Executado
// dentro do OnServe, quando o app e as coleções já existem. Falha (ex.: config
// de cifra inconsistente) aborta o boot — não subir o servidor sem cifra.
func buildServices(a core.App) (*service.Services, error) {
	// Loga a origem (arquivo|env|default) de cada config SEM vazar segredo: só
	// a chave lógica e a origem. Ignora o erro de carga aqui porque a cifra —
	// único ponto fail-closed — é revalidada por cipherConfigLoader() abaixo.
	if _, origins, cerr := configLoader(); cerr == nil {
		for _, o := range origins {
			a.Logger().Info("config carregada", "chave", o.Key, "origem", o.Source)
		}
	}

	cfg, err := cipherConfigLoader()
	if err != nil {
		return nil, err
	}
	kp, err := crypto.NewMapKeyProvider(cfg.CurrentKeyID, cfg.Keys)
	if err != nil {
		return nil, err
	}

	// Observabilidade Prometheus (R8): registry próprio, criado SEMPRE e ANTES da
	// cifra (que é decorada para contar seal/open). O listener só sobe com
	// metrics.addr (api.go). DevIdentity alimenta identidade_dev_ativa (alerta:
	// deve ser 0 em produção); AIEnabled alimenta gemini_ligada; GoogleOK usa o
	// MESMO critério do /readyz (late-binding: googleAuth é criado logo abaixo).
	devActive, aiEnabled := false, false
	if c, _, cerr := configLoader(); cerr == nil {
		devActive = strings.TrimSpace(c.Pangolin.DevIdentity) != ""
		aiEnabled = c.Gemini.Enabled()
	}
	var googleAuth *app.GoogleAuthService
	obs := metrics.New(metrics.Config{
		Build:       buildInfo(),
		DevIdentity: devActive,
		AIEnabled:   aiEnabled,
		Queues:      pb.NewQueueStats(a),
		GoogleOK: func() bool {
			if googleAuth == nil {
				return false
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			state, _ := googleAuth.Status(ctx)
			return state == app.GoogleAuthOK
		},
	})
	cipher := metrics.WrapCipher(crypto.New(kp), obs.App())

	// Serviço de autorização OAuth do Google (R2): fonte única do refresh token
	// (repo cifrado com precedência sobre o env de dev) e do estado de
	// reautorização exposto em /readyz. Construído cedo para alimentar o wiring
	// de Calendar/Drive abaixo.
	googleTokenRepo := pb.NewGoogleTokenRepository(a, cipher)
	googleAuth = app.NewGoogleAuthService(googleTokenRepo)

	// Repositório da pasta de DR (R3): persiste o id da pasta criada/descoberta
	// automaticamente (singleton drive_dr). Id em claro (não é segredo).
	driveFolderRepo := pb.NewDriveFolderRepository(a)

	// Hook de DR no ciclo de backup (Fase 7): espelha o snapshot cifrado no
	// Drive (fallback/DR, fora do caminho de request). Degrada para no-op sem
	// credenciais Google — o backup local segue. A pasta é resolvida
	// automaticamente (R3): drive_folder deixou de ser obrigatório.
	driveStatus := &drivesync.Status{}
	driveSyncWirer(a, kp, googleAuth, driveFolderRepo, obs.Drive(), driveStatus)

	// Repositórios (adapter platform/pb).
	patientsRepo := pb.NewPatientRepository(a)
	originsRepo := pb.NewOriginRepository(a)
	sessionsRepo := pb.NewSessionRepository(a)
	appointmentsRepo := pb.NewAppointmentRepository(a)
	locationsRepo := pb.NewLocationRepository(a)
	plansRepo := pb.NewPlanRepository(a)
	debtsRepo := pb.NewDebtRepository(a)
	paymentsRepo := pb.NewPaymentRepository(a)
	costsRepo := pb.NewCostRepository(a)
	reportsRepo := pb.NewReportRepository(a)
	gedRepo := pb.NewGEDMetaRepository(a)
	portalAuth := pb.NewPortalAuthRepository(a)
	auditRepo := pb.NewAuditRepository(a)
	prontuarioRepo := pb.NewProntuarioRepository(a, cipher)
	aiSummaryRepo := pb.NewAISummaryRepository(a, cipher)
	tagIndexRepo := pb.NewTagIndexRepository(a)
	therapistRepo := pb.NewTherapistRepository(a)

	// Serviços.
	audit := app.NewAuditService(auditRepo)
	audit.SetLogger(a.Logger()) // falha de gravação cai no log do PocketBase (R1)
	audit.SetMetrics(obs.App())
	patients := app.NewPatientService(patientsRepo)
	patients.SetMetrics(obs.App())

	sessions := app.NewSessionService(sessionsRepo, patientsRepo)
	sessions.SetAudit(audit)
	sessions.SetMetrics(obs.App())
	calendarWirer(a, sessions, googleAuth)
	// Mede freebusy/create_event das integrações JÁ injetadas (se houver Google).
	sessions.WrapCalendar(
		func(c app.ConflictChecker) app.ConflictChecker { return metrics.WrapConflictChecker(c, obs.App()) },
		func(c app.SessionCalendar) app.SessionCalendar { return metrics.WrapSessionCalendar(c, obs.App()) },
	)

	billing := app.NewBillingService(debtsRepo, plansRepo, audit)
	billing.SetMetrics(obs.App())
	sessions.AddFinishedHook(billing)

	costSvc := app.NewCostService(costsRepo, locationsRepo, sessionsRepo)
	sessions.AddFinishedHook(costSvc)

	payments := app.NewPaymentService(paymentsRepo, debtsRepo, audit)
	payments.SetMetrics(obs.App())
	payments.SetGED(gedRepo, pb.NewGEDBlobStore(a)) // comprovante de pagamento (R1.9)
	reports := app.NewReportService(reportsRepo, originsRepo, costsRepo)
	sessionMetrics := app.NewSessionMetricsService(sessionsRepo, debtsRepo, locationsRepo)
	dayfeed := app.NewDayFeedService(sessionsRepo, debtsRepo, paymentsRepo, gedRepo, appointmentsRepo, patientsRepo, audit)
	history := app.NewHistoryService(patientsRepo, sessionsRepo, prontuarioRepo, debtsRepo, paymentsRepo, gedRepo, audit)
	prontuario := app.NewProntuarioService(prontuarioRepo, nil)
	prontuario.SetAudit(audit) // R1: audita leitura/escrita do prontuário (ator pelo context)

	// Perfil do terapeuta (R3): serviço singleton 1:1 + injeção do cabeçalho na
	// cobrança (nome/CRP no PDF quando o perfil está configurado).
	therapist := app.NewTherapistService(therapistRepo)

	// Núcleo utilizável (R1.7/R1.9): locais+disponibilidades, origens e planos.
	// Com um plano por sessão vigente, o débito passa a funcionar ponta a ponta
	// via API (BillingService.OnSessionFinished já lê o plano vigente).
	locations := app.NewLocationService(locationsRepo)
	origins := app.NewOriginService(originsRepo)
	plans := app.NewPlanService(plansRepo)
	// Pedidos de agendamento (R1.8): confirmar cria a sessão agendada (conflito
	// + evento de Calendar via SessionService).
	appts := app.NewAppointmentService(appointmentsRepo, patientsRepo, locationsRepo, sessions)
	appts.SetMetrics(obs.App())

	// PDF de cobrança no GED (file storage do PocketBase, D9b): o blob store
	// grava os bytes; o gedRepo guarda o metadado (dedup por sha256). O perfil
	// do terapeuta alimenta o cabeçalho do PDF (R3.3).
	invoice := app.NewInvoiceService(debtsRepo, patientsRepo, gedRepo, pb.NewGEDBlobStore(a), audit)
	invoice.SetMetrics(obs.App())
	invoice.SetTherapist(therapistRepo)

	// IA Gemini (Fase 8): opt-in pela presença de llm.gemini.api_key. Sem chave,
	// aiSummarizerBuilder devolve nil e o AIService fica desligado (Enabled()
	// false). Resumo revisável + índice invertido de tags por paciente/ano.
	ai := app.NewAIService(metrics.WrapAI(aiSummarizerBuilder(a), obs.App()), aiSummaryRepo, tagIndexRepo, sessionsRepo, audit)

	// Credenciais do app Google para o fluxo de autorização OAuth (rotas admin).
	// Origem é o internal/config (arquivo); vazio => fluxo indisponível (422).
	var oauthCfg service.GoogleOAuthConfig
	if c, _, cerr := configLoader(); cerr == nil {
		oauthCfg = service.GoogleOAuthConfig{ClientID: c.Google.ClientID, ClientSecret: c.Google.ClientSecret}
	}

	return &service.Services{
		Patients:    patients,
		Sessions:    sessions,
		Billing:     billing,
		Payments:    payments,
		Costs:       costSvc,
		Reports:     reports,
		Metrics:     sessionMetrics,
		DayFeed:     dayfeed,
		History:     history,
		Audit:       audit,
		Prontuario:  prontuario,
		Invoice:     invoice,
		AI:          ai,
		Therapist:   therapist,
		GoogleAuth:  googleAuth,
		Locations:   locations,
		Origins:     origins,
		Plans:       plans,
		Appts:       appts,
		PortalAuth:  portalAuth,
		Cipher:      cipher,
		DriveStatus: driveStatus,
		Obs:         obs,
		OAuthState:  app.NewMemoryOAuthStateStore(oauthStateClock, app.OAuthStateTTL, app.OAuthStateMaxPending),
		GoogleOAuth: oauthCfg,
	}, nil
}

// resolveGoogleRefreshToken resolve o refresh token do Google com PRECEDÊNCIA DO
// REPO (⟨decisão R2⟩): o token persistido cifrado em google_token é o caminho
// primário; o env PSICOMAN_GOOGLE_REFRESH_TOKEN é um fallback só de dev, usado
// apenas quando o repo está vazio. A origem é logada SEM vazar o segredo (só a
// origem). Devolve ("", false) quando nenhuma fonte tem token.
func resolveGoogleRefreshToken(a core.App, googleAuth *app.GoogleAuthService) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if tok, ok, err := googleAuth.RefreshToken(ctx); err != nil {
		a.Logger().Warn("Google: falha ao ler o refresh token do repositório", "erro", err.Error())
	} else if ok {
		a.Logger().Info("Google: refresh token carregado", "origem", "repositorio (google_token)")
		return tok, true
	}
	if env := strings.TrimSpace(os.Getenv(EnvGoogleRefreshToken)); env != "" {
		a.Logger().Info("Google: refresh token carregado", "origem", "env (fallback de dev)")
		return env, true
	}
	return "", false
}

// calendarWirer é o seam de injeção do Calendar no SessionService. Default liga
// o Google real (wireCalendar); os testes em processo o reatribuem para injetar
// um ConflictChecker/Calendar fake sem tocar a rede (FEAT-002).
var calendarWirer = wireCalendar

// driveSyncWirer é o seam de registro do hook de DR (Drive) no ciclo de backup.
// Default liga o Google real (wireDriveSync); os testes em processo o reatribuem
// para injetar um uploader fake ou desativar, sem tocar a rede (Fase 7).
var driveSyncWirer = wireDriveSync

// buildInfo lê a versão do módulo e do Go para o gauge build_info.
func buildInfo() metrics.BuildInfo {
	bi := metrics.BuildInfo{Version: "desconhecida", GoVersion: runtime.Version()}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		bi.Version = info.Main.Version
	}
	return bi
}

// oauthStateClock é o relógio do store de `state` do OAuth. nil = relógio do
// sistema. Seam de teste: permite expirar um state sem esperar o TTL.
var oauthStateClock clock.Clock

// snapshotAppProps marca os snapshots de DR no Drive (usado no upload e como
// filtro da listagem de retenção).
var snapshotAppProps = map[string]string{"psicoman_kind": "snapshot"}

// driveUploaderAdapter adapta um google.DriveClient à porta
// drivesync.SnapshotUploader. A pasta de DR é resolvida pelo FolderResolver
// (config > persistido > busca > criação), não mais por um folderID fixo. A
// conversão de tipos/erros google<->drivesync mora aqui (o drivesync não
// importa integration/google).
type driveUploaderAdapter struct {
	client   google.DriveClient
	resolver *drivesync.FolderResolver
}

var _ drivesync.SnapshotUploader = (*driveUploaderAdapter)(nil)

// Upload resolve a pasta de DR e envia o snapshot cifrado com as appProperties
// de snapshot. Se o Drive responde 404 na pasta (pasta persistida obsoleta),
// invalida o resolver e refaz a resolução UMA única vez (busca/criação) e o
// upload. Com a pasta vinda do config, o Invalidate é no-op: o 404 vira erro
// observável (o operador definiu a pasta; não recriamos).
func (a *driveUploaderAdapter) Upload(ctx context.Context, name string, content []byte) (string, error) {
	id, err := a.resolver.ID(ctx)
	if err != nil {
		return "", err
	}
	fileID, err := a.uploadTo(ctx, id, name, content)
	if err == nil {
		return fileID, nil
	}
	if !errors.Is(err, drivesync.ErrFolderNotFound) {
		return "", err
	}
	// Pasta obsoleta: invalida e refaz UMA vez.
	a.resolver.Invalidate(id)
	id2, rerr := a.resolver.ID(ctx)
	if rerr != nil {
		return "", rerr
	}
	return a.uploadTo(ctx, id2, name, content)
}

var _ drivesync.SnapshotStore = (*driveUploaderAdapter)(nil)

// List devolve só os snapshots do app (appProperties psicoman_kind=snapshot) na
// pasta de DR resolvida, mais recentes primeiro. A conversão
// google.DriveFile -> drivesync.File mora aqui (o drivesync não importa google).
func (a *driveUploaderAdapter) List(ctx context.Context) ([]drivesync.File, error) {
	id, err := a.resolver.ID(ctx)
	if err != nil {
		return nil, err
	}
	files, err := a.client.List(ctx, google.DriveListInput{FolderID: id, AppProps: snapshotAppProps})
	if err != nil {
		return nil, err
	}
	out := make([]drivesync.File, 0, len(files))
	for _, f := range files {
		out = append(out, drivesync.File{ID: f.ID, Name: f.Name, Created: f.CreatedTime})
	}
	return out, nil
}

// Trash move o snapshot para a lixeira (reversível). Delega ao cliente.
func (a *driveUploaderAdapter) Trash(ctx context.Context, fileID string) error {
	return a.client.Trash(ctx, fileID)
}

// uploadTo faz o upload na pasta folderID e traduz google.ErrDriveNotFound em
// drivesync.ErrFolderNotFound (fronteira de pacote).
func (a *driveUploaderAdapter) uploadTo(ctx context.Context, folderID, name string, content []byte) (string, error) {
	res, err := a.client.Upload(ctx, google.DriveUploadInput{
		Name:     name,
		FolderID: folderID,
		MIME:     "application/octet-stream",
		Content:  content,
		AppProps: snapshotAppProps,
	})
	if err != nil {
		if errors.Is(err, google.ErrDriveNotFound) {
			return "", fmt.Errorf("%w: %v", drivesync.ErrFolderNotFound, err)
		}
		return "", err
	}
	return res.FileID, nil
}

// driveFolderClientAdapter adapta google.DriveClient à drivesync.FolderClient,
// traduzindo google.ErrDriveNotFound em drivesync.ErrFolderNotFound no GetFolder.
type driveFolderClientAdapter struct {
	client google.DriveClient
}

var _ drivesync.FolderClient = (*driveFolderClientAdapter)(nil)

func (a *driveFolderClientAdapter) FindFolder(ctx context.Context, name string, appProps map[string]string) (string, bool, error) {
	return a.client.FindFolder(ctx, name, appProps)
}

func (a *driveFolderClientAdapter) CreateFolder(ctx context.Context, name string, appProps map[string]string) (string, error) {
	return a.client.CreateFolder(ctx, name, appProps)
}

func (a *driveFolderClientAdapter) GetFolder(ctx context.Context, id string) (bool, error) {
	trashed, err := a.client.GetFolder(ctx, id)
	if errors.Is(err, google.ErrDriveNotFound) {
		return false, fmt.Errorf("%w: %v", drivesync.ErrFolderNotFound, err)
	}
	return trashed, err
}

// driveFolderStoreAdapter adapta app.DriveFolderRepository à drivesync.FolderStore,
// convertendo app.ErrNotFound em drivesync.ErrNoStoredFolder (o drivesync não
// importa app).
type driveFolderStoreAdapter struct {
	repo app.DriveFolderRepository
}

var _ drivesync.FolderStore = (*driveFolderStoreAdapter)(nil)

func (a *driveFolderStoreAdapter) Load(ctx context.Context) (string, error) {
	id, err := a.repo.Load(ctx)
	if errors.Is(err, app.ErrNotFound) {
		return "", drivesync.ErrNoStoredFolder
	}
	return id, err
}

func (a *driveFolderStoreAdapter) Save(ctx context.Context, id string) error {
	return a.repo.Save(ctx, id)
}

// geminiSummarizerAdapter adapta um gemini.Summarizer à porta app.AISummarizer
// (desacopla o app do SDK/HTTP do Gemini). Traduz os tipos de borda do app para
// os do pacote de integração e de volta.
type geminiSummarizerAdapter struct {
	client gemini.Summarizer
}

var _ app.AISummarizer = (*geminiSummarizerAdapter)(nil)

func (a *geminiSummarizerAdapter) Summarize(ctx context.Context, in app.AISummaryInput) (*app.AISummaryOutput, error) {
	res, err := a.client.Summarize(ctx, gemini.SummaryInput{Text: in.Text})
	if err != nil {
		return nil, err
	}
	return &app.AISummaryOutput{Summary: res.Summary, Tags: res.Tags}, nil
}

// aiSummarizerBuilder é o seam que resolve o Summarizer da IA a partir do
// Config. Default: liga o Gemini real pela PRESENÇA de llm.gemini.api_key
// (opt-in, R10.3); sem chave devolve nil (IA desligada). Os testes em processo
// o reatribuem para injetar um fake sem tocar a rede.
var aiSummarizerBuilder = buildAISummarizer

// buildAISummarizer liga o Gemini real quando há api_key (opt-in). A chave vem
// do Config (internal/config), NUNCA hard-coded e NUNCA logada
// (psicoman-seguranca-lgpd.md). Sem chave → nil (AIService.Enabled() == false).
func buildAISummarizer(a core.App) app.AISummarizer {
	cfg, _, err := configLoader()
	if err != nil {
		a.Logger().Warn("IA (Gemini) desativada: falha ao ler config", "erro", err.Error())
		return nil
	}
	if !cfg.Gemini.Enabled() {
		a.Logger().Info("IA (Gemini) desativada: sem llm.gemini.api_key (opt-in pela presença da chave)")
		return nil
	}
	a.Logger().Info("IA (Gemini) ativada (opt-in pela presença da api_key)")
	return &geminiSummarizerAdapter{
		client: gemini.NewRESTSummarizer(http.DefaultClient, cfg.Gemini.APIKey),
	}
}

// wireDriveSync registra o hook de DR no ciclo de backup (Fase 7). Ativa o
// espelho real no Drive SÓ quando há credenciais completas (client_id +
// client_secret + drive_folder) e um refresh token; caso contrário registra o
// Syncer com uploader nil — o hook roda como no-op observável (backup local
// segue, sem 500, com aviso logado). O snapshot é cifrado com o BlobSealer
// (mesmas chaves da cifra de campo) antes do upload (drive.file, D9).
func wireDriveSync(a core.App, kp *crypto.MapKeyProvider, googleAuth *app.GoogleAuthService, folders app.DriveFolderRepository, obs drivesync.Observer, st *drivesync.Status) {
	sealer := crypto.NewBlobSealer(kp)
	cfg, _, err := configLoader()
	if err != nil {
		a.Logger().Warn("Drive DR desativado: falha ao ler config", "erro", err.Error())
		drivesync.New(nil, sealer, 0, obs, st).Register(a)
		return
	}
	g := cfg.Google
	// drive_folder NÃO é mais exigido (R3): sem config, a pasta é criada/descoberta
	// automaticamente pelo FolderResolver. Só client_id/secret + refresh token são
	// obrigatórios para ativar o espelho.
	if strings.TrimSpace(g.ClientID) == "" ||
		strings.TrimSpace(g.ClientSecret) == "" {
		a.Logger().Warn("Drive DR desativado: credenciais ausentes (client_id/client_secret) — backup não espelhado")
		drivesync.New(nil, sealer, 0, obs, st).Register(a)
		return
	}
	// Refresh token com precedência do repo (⟨R2⟩); env é fallback de dev.
	refresh, ok := resolveGoogleRefreshToken(a, googleAuth)
	creds := google.OAuthCredentials{ClientID: g.ClientID, ClientSecret: g.ClientSecret, RefreshToken: refresh}
	if !ok || !creds.Valid() {
		googleAuth.FlagReauthorization("Sem refresh token OAuth do Google (reautorização necessária).")
		a.Logger().Warn("Drive DR desativado: sem refresh token OAuth (reautorização necessária) — backup não espelhado")
		drivesync.New(nil, sealer, 0, obs, st).Register(a)
		return
	}
	httpClient := google.NewOAuthHTTPClientWithScopes(context.Background(), creds, google.DriveScope)
	client := google.NewRESTDriveClient(httpClient)
	resolver := drivesync.NewFolderResolver(
		strings.TrimSpace(g.DriveFolder),
		&driveFolderClientAdapter{client: client},
		&driveFolderStoreAdapter{repo: folders},
	)
	uploader := &driveUploaderAdapter{client: client, resolver: resolver}
	drivesync.New(uploader, sealer, g.DriveRetention, obs, st).Register(a)
	a.Logger().Info("Drive DR ativado (hook de backup)",
		"drive_folder_configurado", strings.TrimSpace(g.DriveFolder) != "",
		"retencao", g.DriveRetention)
}

// wireCalendar liga o Google Calendar síncrono ao SessionService a partir do
// Config carregado (FEAT-001). Ativa o Calendar real SÓ quando há credenciais
// completas (client_id + client_secret + calendar_id) e um refresh token; caso
// contrário, NÃO injeta nada: a sessão agenda sem evento (no-op, sem panic),
// com AVISO logado no boot — sem vazar segredo (psicoman-google-api.md D8,
// psicoman-seguranca-lgpd.md). É o único Google no caminho de request.
func wireCalendar(a core.App, sessions *app.SessionService, googleAuth *app.GoogleAuthService) {
	cfg, _, err := configLoader()
	if err != nil {
		a.Logger().Warn("Google Calendar desativado: falha ao ler config", "erro", err.Error())
		return
	}
	g := cfg.Google
	if strings.TrimSpace(g.ClientID) == "" ||
		strings.TrimSpace(g.ClientSecret) == "" ||
		strings.TrimSpace(g.CalendarID) == "" {
		a.Logger().Warn("Google Calendar desativado: credenciais ausentes (client_id/client_secret/calendar_id) — sessões agendam sem evento")
		return
	}
	// Refresh token com precedência do repo (⟨R2⟩); env é fallback de dev.
	refresh, ok := resolveGoogleRefreshToken(a, googleAuth)
	creds := google.OAuthCredentials{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		RefreshToken: refresh,
	}
	if !ok || !creds.Valid() {
		googleAuth.FlagReauthorization("Sem refresh token OAuth do Google (reautorização necessária).")
		a.Logger().Warn("Google Calendar desativado: sem refresh token OAuth (reautorização necessária) — sessões agendam sem evento")
		return
	}
	httpClient := google.NewOAuthHTTPClient(context.Background(), creds)
	adapter := google.NewCalendarAdapter(google.NewRESTCalendarClient(httpClient), g.CalendarID)
	sessions.SetConflictChecker(adapter)
	sessions.SetCalendar(adapter, calendarReminderMinutes)
	a.Logger().Info("Google Calendar ativado (síncrono)", "calendar_id", g.CalendarID)
}
