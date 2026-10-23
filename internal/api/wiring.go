package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/pb"
)

// cipherConfig é a configuração de cifra resolvida no boot.
type cipherConfig struct {
	CurrentKeyID string
	Keys         []crypto.KeySpec
}

// cipherConfigLoader resolve a config de cifra. Default lê do ambiente
// (loadCipherConfig); os testes em processo trocam por um loader determinístico
// com a KeySpec fixa, sem depender de env vars globais (§7).
var cipherConfigLoader = loadCipherConfig

// Variáveis de ambiente da config de cifra (PA-4).
const (
	envCipherKeyID = "PSICOMAN_CIPHER_KEY_ID"
	envCipherKeys  = "PSICOMAN_CIPHER_KEYS"
)

// loadCipherConfig lê a config de cifra do ambiente:
//   - PSICOMAN_CIPHER_KEY_ID: id da chave corrente.
//   - PSICOMAN_CIPHER_KEYS: lista "id:base64key" separada por vírgula.
//
// Ausência/formato inválido → erro, abortando o boot (falhar cedo). O base64 e
// o tamanho (32 bytes) são validados por NewMapKeyProvider.
func loadCipherConfig() (cipherConfig, error) {
	keyID := strings.TrimSpace(os.Getenv(envCipherKeyID))
	raw := strings.TrimSpace(os.Getenv(envCipherKeys))
	if keyID == "" || raw == "" {
		return cipherConfig{}, fmt.Errorf("config de cifra ausente: defina %s e %s", envCipherKeyID, envCipherKeys)
	}
	var specs []crypto.KeySpec
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, b64, ok := strings.Cut(part, ":")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(b64) == "" {
			return cipherConfig{}, fmt.Errorf("entrada de chave de cifra inválida (esperado id:base64): %q", part)
		}
		specs = append(specs, crypto.KeySpec{ID: strings.TrimSpace(id), B64Key: strings.TrimSpace(b64)})
	}
	return cipherConfig{CurrentKeyID: keyID, Keys: specs}, nil
}

// buildServices monta repos + cipher + serviços + hooks de sessão. Executado
// dentro do OnServe, quando o app e as coleções já existem. Falha (ex.: config
// de cifra inconsistente) aborta o boot — não subir o servidor sem cifra.
func buildServices(a core.App) (*service.Services, error) {
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
