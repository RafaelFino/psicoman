// Package app é a camada de bootstrap dos dois binários (admin e portal).
// Carrega config, inicializa plataforma (logger, métricas, storage) e monta o
// servidor HTTP com as rotas de cada superfície.
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/admin"
	"github.com/RafaelFino/psicoman/internal/api/portal"
	"github.com/RafaelFino/psicoman/internal/api/swagger"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/migration"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/logger"
	"github.com/RafaelFino/psicoman/internal/platform/metrics"
	"github.com/RafaelFino/psicoman/internal/repository/ged"
	"github.com/RafaelFino/psicoman/internal/repository/sqlite"
	"github.com/RafaelFino/psicoman/internal/service"
	"github.com/RafaelFino/psicoman/internal/web"
)

// Runnable é um servidor montado, pronto para rodar até o contexto ser cancelado.
type Runnable interface {
	Run(ctx context.Context) error
}

// Options carrega dependências injetáveis (usado nos testes E2E para fornecer
// clientes Google mockados). Em produção, ficam nil e o app usa a integração
// real quando o Google está autorizado.
type Options struct {
	Calendar service.CalendarClient
	Gmail    service.GmailClient
	Drive    service.DriveClient
	Identity portal.IdentityVerifier
}

// instance implementa Runnable sobre um api.Server.
type instance struct {
	log *logger.Logger
	srv *api.Server
	db  *sqlite.DB
}

// Run sobe o servidor e aguarda o cancelamento do contexto para encerrar.
func (i *instance) Run(ctx context.Context) error {
	if i.db != nil {
		defer i.db.Close()
	}
	return serve(ctx, i.log, i.srv)
}

// deps agrupa as dependências de plataforma compartilhadas pelos binários.
type deps struct {
	cfg     *config.Config
	log     *logger.Logger
	metrics *metrics.Registry
	health  *api.Health
	db      *sqlite.DB
}

// bootstrap inicializa a plataforma comum a partir de um config já carregado:
// logger, métricas, banco SQLite (WAL) e aplicação das migrations no boot.
func bootstrap(cfg *config.Config, component string) (*deps, error) {
	log, err := logger.New(logger.Options{
		Level:     cfg.Log.Level,
		Dir:       cfg.Paths.LogDir,
		Component: component,
	})
	if err != nil {
		return nil, err
	}

	db, err := sqlite.Open(sqlite.Options{Path: cfg.Paths.SQLite})
	if err != nil {
		log.Close()
		return nil, fmt.Errorf("abrindo banco: %w", err)
	}

	if err := migration.Run(db.DB); err != nil {
		_ = db.Close()
		log.Close()
		return nil, fmt.Errorf("aplicando migrations: %w", err)
	}
	log.Info("banco de dados pronto e migrations aplicadas", "arquivo", cfg.Paths.SQLite)

	reg := metrics.New()
	health := api.NewHealth(reg)
	health.AddReadiness("sqlite", api.HealthCheckFunc(db.Ping))

	return &deps{cfg: cfg, log: log, metrics: reg, health: health, db: db}, nil
}

// chainMiddleware compõe middlewares aplicando-os na ordem informada: o
// primeiro é o mais externo (roda primeiro na requisição).
func chainMiddleware(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// serve sobe o servidor e aguarda o contexto ser cancelado para encerrar.
func serve(ctx context.Context, log *logger.Logger, srv *api.Server) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case err := <-errCh:
		if err != nil {
			log.Error("servidor encerrou com erro", "error", err)
			return err
		}
		return nil
	case <-ctx.Done():
		log.Info("encerrando servidor")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("erro no shutdown", "error", err)
			return err
		}
		return nil
	}
}

// buildAdmin monta o servidor admin (rotas + dependências).
func buildAdmin(ctx context.Context, cfg *config.Config, opts Options) (*instance, error) {
	d, err := bootstrap(cfg, "admin")
	if err != nil {
		return nil, err
	}
	srv := api.NewServer(cfg.Admin.Addr(), d.log, d.metrics)
	d.health.Register(srv.Mux())
	swagger.Register(srv.Mux(), swagger.Admin)
	if webSrv, werr := web.New(web.Admin); werr == nil {
		webSrv.Register(srv.Mux())
	} else {
		return nil, fmt.Errorf("web admin: %w", werr)
	}

	// Serviços compartilhados.
	auditSvc := service.NewAuditService(sqlite.NewAuditRepo(d.db))
	patientSvc := service.NewPatientService(sqlite.NewPatientRepo(d.db))
	originSvc := service.NewOriginService(sqlite.NewOriginRepo(d.db))
	locationSvc := service.NewLocationService(sqlite.NewLocationRepo(d.db))

	gedStore, err := ged.NewStore(cfg.Paths.GEDRoot)
	if err != nil {
		return nil, fmt.Errorf("inicializando GED: %w", err)
	}
	gedSvc := service.NewGEDService(sqlite.NewGEDMetaRepo(d.db), gedStore)
	therapistSvc := service.NewTherapistService(sqlite.NewTherapistRepo(d.db), gedSvc)
	sessionSvc := service.NewSessionService(sqlite.NewSessionRepo(d.db), sqlite.NewPatientRepo(d.db))
	planSvc := service.NewPlanService(sqlite.NewPlanRepo(d.db), sqlite.NewPatientRepo(d.db))
	billingSvc := service.NewBillingService(sqlite.NewDebtRepo(d.db), sqlite.NewPlanRepo(d.db), auditSvc)
	invoiceSvc := service.NewInvoiceService(sqlite.NewDebtRepo(d.db), sqlite.NewPatientRepo(d.db), gedSvc, therapistSvc, auditSvc)
	paymentSvc := service.NewPaymentService(sqlite.NewPaymentRepo(d.db), sqlite.NewDebtRepo(d.db), gedSvc, auditSvc)
	prontuarioSvc := service.NewProntuarioService(sqlite.NewProntuarioRepo(d.db), sqlite.NewTemplateRepo(d.db), sqlite.NewPatientRepo(d.db))
	costRepo := sqlite.NewCostRepo(d.db)
	costSvc := service.NewCostService(costRepo, sqlite.NewLocationRepo(d.db), costRepo)
	reportSvc := service.NewReportService(sqlite.NewReportRepo(d.db), sqlite.NewOriginRepo(d.db), costRepo)
	// Custos reagem à finalização da sessão (atribuição direta ou rateio).
	sessionSvc.AddFinishedHook(costSvc)
	// O financeiro reage à finalização da sessão (débito por sessão, idempotente).
	sessionSvc.AddFinishedHook(billingSvc)

	// --- Integração Google (Calendar/Meet, Gmail) ---
	cipher := crypto.New(crypto.StaticKeyProvider{B64: cfg.Crypto.Key})
	tokenRepo := sqlite.NewGoogleTokenRepo(d.db, cipher)
	oauth := google.NewOAuth(google.OAuthConfig{
		ClientID:     cfg.Google.ClientID,
		ClientSecret: cfg.Google.ClientSecret,
		RedirectURL:  cfg.Google.RedirectURL,
		Scopes:       cfg.Google.Scopes,
	}, tokenRepo)

	// Clientes Google: mock injetado (testes) ou real (OAuth).
	var calendar service.CalendarClient = opts.Calendar
	var gmail service.GmailClient = opts.Gmail
	var drive service.DriveClient = opts.Drive
	if calendar == nil || gmail == nil || drive == nil {
		real := google.NewClient(oauth, cfg.Google.CalendarID)
		if calendar == nil {
			calendar = real
		}
		if gmail == nil {
			gmail = real
		}
		if drive == nil {
			drive = real
		}
	}
	// Checagem de conflito (freebusy) + criação de evento ao agendar.
	sessionSvc.SetConflictChecker(service.NewCalendarConflictChecker(calendar))
	sessionSvc.SetCalendar(calendar, cfg.Reminders.MinutesBefore)
	// Envio de templates e cobranças por email (best-effort).
	prontuarioSvc.SetSender(service.NewGmailTemplateSender(gmail))
	invoiceSvc.SetGmail(gmail)
	// Readiness: reautorização Google pendente = degraded.
	d.health.AddReadiness("google", api.HealthCheckFunc(func() error {
		req, _ := tokenRepo.ReauthRequired(context.Background())
		if req {
			return fmt.Errorf("reautorização do Google pendente")
		}
		return nil
	}))

	// Autenticação admin (defense in depth sobre o Pangolin).
	authn := admin.NewAuthenticator(cfg.AdminAuth, auditSvc, d.log)
	if cfg.Dev {
		authn.EnableDev()
		d.log.Warn("MODO DEV ATIVO: autenticação do admin DESLIGADA. Nunca use em produção.")
	}

	// Router versionado /v1.
	v1 := api.V1(srv.Mux())
	// /version é informativo e público (sem auth de negócio).
	api.RegisterVersion(v1.Admin(), "admin")
	// Rotas de negócio do admin exigem autenticação (Pangolin + validação própria).
	adminGroup := v1.Admin().WithAuth(authn.Middleware)
	admin.NewHandlers(cfg, auditSvc).Register(adminGroup)
	admin.NewPatientHandlers(patientSvc, auditSvc).Register(adminGroup)
	admin.NewOriginHandlers(originSvc, auditSvc).Register(adminGroup)
	admin.NewLocationHandlers(locationSvc, auditSvc).Register(adminGroup)
	admin.NewGEDHandlers(gedSvc, patientSvc, auditSvc).Register(adminGroup)
	admin.NewTherapistHandlers(therapistSvc, auditSvc).Register(adminGroup)
	admin.NewSessionHandlers(sessionSvc, auditSvc).Register(adminGroup)
	admin.NewPlanHandlers(planSvc, auditSvc).Register(adminGroup)
	admin.NewDebtHandlers(billingSvc, invoiceSvc, auditSvc).Register(adminGroup)
	admin.NewPaymentHandlers(paymentSvc, auditSvc).Register(adminGroup)
	admin.NewProntuarioHandlers(prontuarioSvc, auditSvc).Register(adminGroup)
	admin.NewCostHandlers(costSvc, reportSvc, auditSvc).Register(adminGroup)
	admin.NewGoogleHandlers(oauth, auditSvc).Register(adminGroup)

	// Pedidos de agendamento (confirmação cria a sessão + evento).
	apptSvc := service.NewAppointmentService(
		sqlite.NewAppointmentRepo(d.db), sqlite.NewPatientRepo(d.db), sessionSvc, sqlite.NewLocationRepo(d.db))
	admin.NewAppointmentHandlers(apptSvc, auditSvc).Register(adminGroup)

	// Backup/restore cifrado no Drive (SQLite + GED incremental).
	gedMeta := sqlite.NewGEDMetaRepo(d.db)
	gedSource := service.NewGEDManifestSource(gedMeta, gedStore)
	backupSvc := service.NewBackupService(d.db.DB, cfg.Paths.SQLite, cipher, drive, cfg.Google.DriveFolder, gedSource, auditSvc)
	admin.NewBackupHandlers(backupSvc, auditSvc).Register(adminGroup)

	// Job diário de fechamento de ciclo (planos fechados).
	startCycleCloseJob(ctx, d.log, billingSvc)
	// Job diário de backup cifrado no Drive.
	startBackupJob(ctx, d.log, backupSvc)

	return &instance{log: d.log, srv: srv, db: d.db}, nil
}

// buildPortal monta o servidor portal (self-service do paciente).
func buildPortal(_ context.Context, cfg *config.Config, opts Options) (*instance, error) {
	d, err := bootstrap(cfg, "portal")
	if err != nil {
		return nil, err
	}
	srv := api.NewServer(cfg.Portal.Addr(), d.log, d.metrics)
	d.health.Register(srv.Mux())
	swagger.Register(srv.Mux(), swagger.Portal)
	if webSrv, werr := web.New(web.Portal); werr == nil {
		webSrv.Register(srv.Mux())
	} else {
		return nil, fmt.Errorf("web portal: %w", werr)
	}

	// Serviços do portal.
	patientRepo := sqlite.NewPatientRepo(d.db)
	patientSvc := service.NewPatientService(patientRepo)
	portalSessionSvc := service.NewPortalSessionService(patientRepo, sqlite.NewSessionRepo(d.db), sqlite.NewDebtRepo(d.db))
	locationSvc := service.NewLocationService(sqlite.NewLocationRepo(d.db))
	// SessionService sem calendar (o portal só cria pedidos, não sessões).
	portalSessions := service.NewSessionService(sqlite.NewSessionRepo(d.db), patientRepo)
	apptSvc := service.NewAppointmentService(
		sqlite.NewAppointmentRepo(d.db), patientRepo, portalSessions, sqlite.NewLocationRepo(d.db))

	// Sessão própria (cookie assinado com o secret do admin como chave HMAC).
	// Em modo dev (http://localhost) o cookie não pode ser Secure, senão o
	// navegador não o reenvia e as rotas autenticadas do portal dão 401.
	sessMgr := portal.NewSessionManager(cfg.AdminAuth.Secret, 24*time.Hour, !cfg.Dev)
	// Login social: verificador injetável (fake nos testes, Google userinfo real).
	var verifier portal.IdentityVerifier = opts.Identity
	if verifier == nil {
		if cfg.Dev {
			// Modo dev: aceita a credencial como e-mail, sem chamar o Google.
			verifier = google.FakeIdentityVerifier{}
			d.log.Warn("MODO DEV ATIVO: login do portal aceita qualquer e-mail sem verificação. Nunca use em produção.")
		} else {
			verifier = google.NewIdentityVerifier()
		}
	}
	limiter := portal.NewRateLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
	authn := portal.NewAuthenticator(sessMgr)
	gate := portal.NewApprovalGate(patientSvc)
	handlers := portal.NewHandlers(patientSvc, portalSessionSvc, locationSvc, apptSvc, sessMgr, verifier, limiter)

	v1 := api.V1(srv.Mux())
	api.RegisterVersion(v1.Portal(), "portal")
	// Rotas públicas (login/cadastro): rate limit por IP (e por email no handler).
	handlers.RegisterPublic(v1.Portal().WithAuth(limiter.Middleware))
	// Rotas autenticadas. Dois grupos: só-sessão (status/leitura do cadastro) e
	// atrás do gate de aprovação (recursos). O gate roda depois do Authenticator.
	authedGroup := v1.Portal().WithAuth(authn.Middleware)
	gatedGroup := v1.Portal().WithAuth(chainMiddleware(authn.Middleware, gate.Middleware))
	handlers.RegisterAuthenticated(authedGroup, gatedGroup)

	return &instance{log: d.log, srv: srv, db: d.db}, nil
}

// RunAdmin sobe o binário administrativo a partir do arquivo de config.
func RunAdmin(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("carregando config: %w", err)
	}
	inst, err := buildAdmin(ctx, cfg, Options{})
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	defer inst.log.Close()
	return inst.Run(ctx)
}

// RunPortal sobe o binário do portal a partir do arquivo de config.
func RunPortal(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("carregando config: %w", err)
	}
	inst, err := buildPortal(ctx, cfg, Options{})
	if err != nil {
		return fmt.Errorf("bootstrap portal: %w", err)
	}
	defer inst.log.Close()
	return inst.Run(ctx)
}

// NewAdminForTest monta o servidor admin para uso em testes E2E (config em
// memória), com dependências injetáveis (ex: clientes Google mockados).
func NewAdminForTest(ctx context.Context, cfg *config.Config, opts Options) (Runnable, error) {
	return buildAdmin(ctx, cfg, opts)
}

// NewPortalForTest monta o servidor portal para uso em testes E2E.
func NewPortalForTest(ctx context.Context, cfg *config.Config, opts Options) (Runnable, error) {
	return buildPortal(ctx, cfg, opts)
}
