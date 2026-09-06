package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PaymentHandlers expõe a quitação de débitos e anexação de comprovantes.
type PaymentHandlers struct {
	svc   *service.PaymentService
	audit *service.AuditService
}

// NewPaymentHandlers cria os handlers de pagamento.
func NewPaymentHandlers(svc *service.PaymentService, audit *service.AuditService) *PaymentHandlers {
	return &PaymentHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de pagamento no grupo autenticado.
func (h *PaymentHandlers) Register(g *api.Group) {
	g.Handle("POST", "/debts/{id}/pay", h.pay)
	g.Handle("GET", "/debts/{id}/payments", h.list)
	g.Handle("POST", "/payments/{pid}/receipt", h.receipt)
}

type payBody struct {
	Amount int64  `json:"amount"`
	Method string `json:"method"`
}

func paymentView(p *domain.Payment) map[string]any {
	return map[string]any{
		"id":      p.ID,
		"debt_id": p.DebtID,
		"amount":  p.Amount,
		"method":  p.Method,
		"paid_at": p.PaidAt,
	}
}

func (h *PaymentHandlers) pay(w http.ResponseWriter, r *http.Request) {
	var b payBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	debtID := r.PathValue("id")
	debt, pay, err := h.svc.Pay(r.Context(), debtID, service.PayInput{Amount: b.Amount, Method: b.Method})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDebtPay, "debt", debtID,
		map[string]any{"payment_id": pay.ID, "status": debt.Status})
	httpx.Respond(w, r, http.StatusCreated, "Pagamento registrado.", map[string]any{
		"payment":     paymentView(pay),
		"debt_status": debt.Status,
	})
}

func (h *PaymentHandlers) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.ListPayments(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		views = append(views, paymentView(p))
	}
	httpx.Respond(w, r, http.StatusOK, "Pagamentos listados.", views)
}

// receipt anexa um comprovante (multipart "file") a um pagamento.
func (h *PaymentHandlers) receipt(w http.ResponseWriter, r *http.Request) {
	const maxUpload = 25 << 20
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Não foi possível ler o comprovante enviado."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Envie o comprovante no campo 'file'."))
		return
	}
	defer file.Close()

	// O comprovante referencia o débito via query/param; aqui usamos o debt_id do form.
	debtID := r.FormValue("debt_id")
	if debtID == "" {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Informe o campo 'debt_id'."))
		return
	}
	f, err := h.svc.AttachReceipt(r.Context(), debtID, r.PathValue("pid"), header.Header.Get("Content-Type"), file)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDebtPay, "payment_receipt", r.PathValue("pid"),
		map[string]any{"ged_file_id": f.ID})
	httpx.Respond(w, r, http.StatusCreated, "Comprovante anexado.", map[string]any{
		"ged_file_id": f.ID,
	})
}
