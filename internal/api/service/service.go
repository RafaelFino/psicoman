// Package service define o container de serviços (Services) montado pelo wiring
// e consumido pelos handlers de cada ator. Vive num pacote folha para que os
// pacotes de ator (api/app, api/portal, api/admin) possam recebê-lo sem criar
// ciclo de import com o pacote api (que monta o Services e registra as rotas).
package service

import "github.com/RafaelFino/psicoman/internal/app"

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
}
