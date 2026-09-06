package admin

import (
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// DebtHandlers expõe listagem de débitos, PDF de cobrança e o fechamento de ciclo.
type DebtHandlers struct {
	billing *service.BillingService
	invoice *service.InvoiceService
	audit   *service.AuditService
}

// NewDebtHandlers cria os handlers de débito.
func NewDebtHandlers(billing *service.BillingService, invoice *service.InvoiceService, audit *service.AuditService) *DebtHandlers {
	return &DebtHandlers{billing: billing, invoice: invoice, audit: audit}
}

// Register instala as rotas de débitos no grupo autenticado.
func (h *DebtHandlers) Register(g *api.Group) {
	g.Handle("GET", "/debts", h.list)
	g.Handle("GET", "/debts/{id}", h.get)
	g.Handle("POST", "/debts/{id}/pdf", h.generatePDF)
	g.Handle("GET", "/debts/{id}/pdf", h.downloadPDF)
	g.Handle("POST", "/debts/{id}/send-email", h.sendEmail)
	g.Handle("POST", "/billing/close-cycles", h.closeCycles)
}

func debtView(d *domain.Debt) map[string]any {
	view := map[string]any{
		"id":             d.ID,
		"patient_id":     d.PatientID,
		"session_id":     d.SessionID,
		"plan_id":        d.PlanID,
		"billing_period": d.BillingPeriod,
		"amount":         d.Amount,
		"status":         d.Status,
		"pdf_file_id":    d.PDFFileID,
		"created_at":     d.CreatedAt,
	}
	if !d.DueDate.IsZero() {
		view["due_date"] = d.DueDate
	}
	return view
}

func (h *DebtHandlers) list(w http.ResponseWriter, r *http.Request) {
	var (
		ds  []*domain.Debt
		err error
	)
	if pid := r.URL.Query().Get("patient_id"); pid != "" {
		ds, err = h.billing.ListDebtsByPatient(r.Context(), pid)
	} else {
		ds, err = h.billing.ListDebts(r.Context())
	}
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ds))
	for _, d := range ds {
		views = append(views, debtView(d))
	}
	httpx.Respond(w, r, http.StatusOK, "Débitos listados.", views)
}

func (h *DebtHandlers) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.billing.GetDebt(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Débito encontrado.", debtView(d))
}

// generatePDF gera (ou regenera) o PDF de cobrança e o armazena no GED.
func (h *DebtHandlers) generatePDF(w http.ResponseWriter, r *http.Request) {
	f, err := h.invoice.GeneratePDF(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "PDF de cobrança gerado.", map[string]any{
		"ged_file_id": f.ID,
		"sha256":      f.SHA256,
	})
}

// sendEmail envia a cobrança por email ao paciente (best-effort).
func (h *DebtHandlers) sendEmail(w http.ResponseWriter, r *http.Request) {
	if err := h.invoice.SendCharge(r.Context(), r.PathValue("id")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Cobrança enviada por email.", nil)
}

// downloadPDF serve o PDF de cobrança (gera on-demand se ainda não existe).
func (h *DebtHandlers) downloadPDF(w http.ResponseWriter, r *http.Request) {
	data, err := h.invoice.GetPDF(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="cobranca.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

type closeCyclesBody struct {
	Date string `json:"date"` // opcional, ISO-8601; default hoje
}

// closeCycles dispara o fechamento de ciclo dos planos fechados (job manual).
func (h *DebtHandlers) closeCycles(w http.ResponseWriter, r *http.Request) {
	ref := clock.Now()
	var b closeCyclesBody
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(r, &b); err == nil && b.Date != "" {
			if t, perr := time.Parse(time.RFC3339, b.Date); perr == nil {
				ref = t
			}
		}
	}
	created, err := h.billing.CloseCycles(r.Context(), ref)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Fechamento de ciclo executado.", map[string]any{
		"debts_created": created,
	})
}
