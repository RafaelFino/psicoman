package api

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
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
	cipher := crypto.New(kp)

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
	auditRepo := pb.NewAuditRepository(a)
	prontuarioRepo := pb.NewProntuarioRepository(a, cipher)

	// Serviços.
	audit := app.NewAuditService(auditRepo)
	patients := app.NewPatientService(patientsRepo)

	sessions := app.NewSessionService(sessionsRepo, patientsRepo)
	sessions.SetAudit(audit)
	calendarWirer(a, sessions)

	billing := app.NewBillingService(debtsRepo, plansRepo, audit)
	sessions.AddFinishedHook(billing)

	costSvc := app.NewCostService(costsRepo, locationsRepo, sessionsRepo)
	sessions.AddFinishedHook(costSvc)

	payments := app.NewPaymentService(paymentsRepo, debtsRepo, audit)
	reports := app.NewReportService(reportsRepo, originsRepo, costsRepo)
	metrics := app.NewSessionMetricsService(sessionsRepo, debtsRepo, locationsRepo)
	dayfeed := app.NewDayFeedService(sessionsRepo, debtsRepo, paymentsRepo, gedRepo, appointmentsRepo, patientsRepo, audit)
	history := app.NewHistoryService(patientsRepo, sessionsRepo, prontuarioRepo, debtsRepo, paymentsRepo, gedRepo, audit)
	prontuario := app.NewProntuarioService(prontuarioRepo, nil)

	// PDF de cobrança no GED (file storage do PocketBase, D9b): o blob store
	// grava os bytes; o gedRepo guarda o metadado (dedup por sha256).
	invoice := app.NewInvoiceService(debtsRepo, patientsRepo, gedRepo, pb.NewGEDBlobStore(a), audit)

	return &service.Services{
		Patients:   patients,
		Sessions:   sessions,
		Billing:    billing,
		Payments:   payments,
		Costs:      costSvc,
		Reports:    reports,
		Metrics:    metrics,
		DayFeed:    dayfeed,
		History:    history,
		Audit:      audit,
		Prontuario: prontuario,
		Invoice:    invoice,
	}, nil
}

// calendarWirer é o seam de injeção do Calendar no SessionService. Default liga
// o Google real (wireCalendar); os testes em processo o reatribuem para injetar
// um ConflictChecker/Calendar fake sem tocar a rede (FEAT-002).
var calendarWirer = wireCalendar

// wireCalendar liga o Google Calendar síncrono ao SessionService a partir do
// Config carregado (FEAT-001). Ativa o Calendar real SÓ quando há credenciais
// completas (client_id + client_secret + calendar_id) e um refresh token; caso
// contrário, NÃO injeta nada: a sessão agenda sem evento (no-op, sem panic),
// com AVISO logado no boot — sem vazar segredo (psicoman-google-api.md D8,
// psicoman-seguranca-lgpd.md). É o único Google no caminho de request.
func wireCalendar(a core.App, sessions *app.SessionService) {
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
	refresh := strings.TrimSpace(os.Getenv(EnvGoogleRefreshToken))
	creds := google.OAuthCredentials{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		RefreshToken: refresh,
	}
	if !creds.Valid() {
		a.Logger().Warn("Google Calendar desativado: sem refresh token OAuth (reautorização necessária) — sessões agendam sem evento")
		return
	}
	httpClient := google.NewOAuthHTTPClient(context.Background(), creds)
	adapter := google.NewCalendarAdapter(google.NewRESTCalendarClient(httpClient), g.CalendarID)
	sessions.SetConflictChecker(adapter)
	sessions.SetCalendar(adapter, calendarReminderMinutes)
	a.Logger().Info("Google Calendar ativado (síncrono)", "calendar_id", g.CalendarID)
}
