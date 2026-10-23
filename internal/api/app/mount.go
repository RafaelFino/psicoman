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

	// Sessões.
	g.POST("/sessions", h.CreateSession)
	g.GET("/sessions/{id}", h.GetSession)
	g.PATCH("/sessions/{id}", h.UpdateSession)
	g.POST("/sessions/{id}/confirm", h.ConfirmSession)
	g.POST("/sessions/{id}/cancel", h.CancelSession)
	g.POST("/sessions/{id}/finish", h.FinishSession)

	// Cobrança.
	g.GET("/debts/{id}", h.GetDebt)
	g.POST("/debts/{id}/pay", h.PayDebt)

	// Relatórios.
	g.GET("/reports/financial", h.ReportFinancial)
	g.GET("/reports/roi", h.ReportROI)
}

// reqCtx deriva um contexto com timeout a partir da requisição.
func reqCtx(e *core.RequestEvent) (context.Context, context.CancelFunc) {
	return context.WithTimeout(e.Request.Context(), handlerTimeout)
}
