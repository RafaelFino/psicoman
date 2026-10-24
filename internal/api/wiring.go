package api

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/pb"
)

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
	}, nil
}
