// Package service define o container de serviços (Services) montado pelo wiring
// e consumido pelos handlers de cada ator. Vive num pacote folha para que os
// pacotes de ator (api/app, api/portal, api/admin) possam recebê-lo sem criar
// ciclo de import com o pacote api (que monta o Services e registra as rotas).
package service

import (
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
)

// Services agrega os serviços de aplicação usados pelos handlers /v1/*. É
// montado por api.buildServices (wiring) dentro do OnServe, quando o app e as
// coleções já existem.
type Services struct {
	Patients   *app.PatientService
	Sessions   *app.SessionService
	Billing    *app.BillingService
	Payments   *app.PaymentService
	Costs      *app.CostService
	Reports    *app.ReportService
	Metrics    *app.SessionMetricsService
	DayFeed    *app.DayFeedService
	History    *app.HistoryService
	Audit      *app.AuditService
	Prontuario *app.ProntuarioService
	Invoice    *app.InvoiceService
	AI         *app.AIService
	Therapist  *app.TherapistService
	GoogleAuth *app.GoogleAuthService
	Locations  *app.LocationService
	Origins    *app.OriginService
	Plans      *app.PlanService
	Appts      *app.AppointmentService
	PortalAuth app.PortalAuth

	// Cipher é a cifra montada no boot, exposta para o readiness check (/readyz)
	// fazer um round-trip seal/open de verificação. Nunca cifra dado real aqui.
	Cipher domain.Cipher

	// GoogleOAuth carrega as credenciais do app (client_id/secret) para o fluxo
	// de autorização 3-legged (rotas admin). Vazio quando não configurado — o
	// handler responde 422 nesse caso. NUNCA logar/serializar o secret.
	GoogleOAuth GoogleOAuthConfig
}

// GoogleOAuthConfig são as credenciais do app Google para o fluxo de
// autorização (montadas pelo wiring a partir do internal/config).
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// Configured indica se há client_id + client_secret para iniciar o fluxo OAuth.
func (c GoogleOAuthConfig) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}
