package app

import (
	"context"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/api/service"
)

// handlerTimeout é o timeout padrão de I/O por handler (psicoman-golang.md:
// todo I/O tem timeout). Default único na Fase 4 (PA-6).
const handlerTimeout = 5 * time.Second

// Handlers agrupa os serviços usados pelos handlers do ator app.
type Handlers struct {
	svc *service.Services
}

// Mount registra as rotas /v1/app/* no grupo informado.
func Mount(g *router.RouterGroup[*core.RequestEvent], svc *service.Services) {
	h := &Handlers{svc: svc}

	// Pacientes + prontuário.
	g.GET("/patients", h.ListPatients)
	g.POST("/patients", h.CreatePatient)
	// patients/pending vence patients/{id} por especificidade do http.ServeMux.
	g.GET("/patients/pending", h.ListPendingPatients)
	g.GET("/patients/{id}", h.GetPatientChart)
	g.PATCH("/patients/{id}", h.UpdatePatient)
	g.DELETE("/patients/{id}", h.DeletePatient)
	g.POST("/patients/{id}/approve", h.ApprovePatient)
	g.POST("/patients/{id}/reject", h.RejectPatient)
	g.POST("/patients/{id}/notes", h.AddNote)
	g.PUT("/patients/{id}/anamnesis", h.UpsertAnamnesis)
	g.GET("/patients/{id}/debts", h.ListPatientDebts)

	// Telas iniciais (R1.1-R1.3): feed do dia, história, métricas de sessão.
	g.GET("/day-feed", h.DayFeed)
	g.GET("/patients/{id}/history", h.PatientHistory)
	g.GET("/patients/{id}/session-metrics", h.PatientSessionMetrics)

	// Sessões.
	g.GET("/sessions", h.ListSessions) // calendário (R1.4): ?patient_id=&from=&to=
	g.POST("/sessions", h.CreateSession)
	g.GET("/sessions/{id}", h.GetSession)
	g.PATCH("/sessions/{id}", h.UpdateSession)
	g.POST("/sessions/{id}/confirm", h.ConfirmSession)
	g.POST("/sessions/{id}/cancel", h.CancelSession)
	g.POST("/sessions/{id}/finish", h.FinishSession)

	// Cobrança.
	g.GET("/debts/{id}", h.GetDebt)
	g.POST("/debts/{id}/pay", h.PayDebt)
	g.POST("/debts/{id}/invoice", h.GenerateInvoice)
	g.GET("/debts/{id}/invoice.pdf", h.DownloadInvoice)

	// IA (Gemini) — resumo clínico revisável + índice de tags (opt-in).
	g.POST("/sessions/{id}/ai-summary", h.SuggestAISummary)
	g.POST("/ai-summaries/{id}/consolidate", h.ConsolidateAISummary)
	g.GET("/patients/{id}/tags", h.SearchByTag)

	// Relatórios.
	g.GET("/reports/financial", h.ReportFinancial)
	g.GET("/reports/roi", h.ReportROI)

	// Perfil do terapeuta (R3) — singleton 1:1 + links de plataforma.
	g.GET("/therapist", h.GetTherapist)
	g.PUT("/therapist", h.UpsertTherapist)
	g.POST("/therapist/platform-links", h.AddTherapistLink)
	g.DELETE("/therapist/platform-links/{id}", h.DeleteTherapistLink)

	// Locais + disponibilidades (R1.7). A rota estática de delete de
	// disponibilidade vem antes de /locations/{id} por especificidade.
	g.GET("/locations", h.ListLocations)
	g.POST("/locations", h.CreateLocation)
	g.DELETE("/locations/availability/{id}", h.DeleteAvailability)
	g.GET("/locations/{id}", h.GetLocation)
	g.PATCH("/locations/{id}", h.UpdateLocation)
	g.DELETE("/locations/{id}", h.DeleteLocation)
	g.GET("/locations/{id}/availability", h.ListAvailability)
	g.POST("/locations/{id}/availability", h.AddAvailability)

	// Origens / canais de aquisição (R1.7).
	g.GET("/origins", h.ListOrigins)
	g.POST("/origins", h.CreateOrigin)
	g.PATCH("/origins/{id}", h.UpdateOrigin)
	g.DELETE("/origins/{id}", h.DeleteOrigin)

	// Planos por paciente (R1.9) — habilitam o débito por sessão via API.
	g.GET("/patients/{id}/plans", h.ListPatientPlans)
	g.POST("/patients/{id}/plans", h.CreatePatientPlan)
	g.DELETE("/plans/{id}", h.DeletePatientPlan)
}

// reqCtx deriva um contexto com timeout a partir da requisição.
func reqCtx(e *core.RequestEvent) (context.Context, context.CancelFunc) {
	return context.WithTimeout(e.Request.Context(), handlerTimeout)
}
