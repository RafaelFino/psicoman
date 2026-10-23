package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
)

type payBody struct {
	AmountCents int64  `json:"amount_cents"`
	Method      string `json:"method"`
}

// GetDebt GET /v1/app/debts/{id}.
func (h *Handlers) GetDebt(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	d, err := h.svc.Billing.GetDebt(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Débito encontrado.", toDebtDTO(d))
}

// PayDebt POST /v1/app/debts/{id}/pay. paid_at é definido pelo serviço (clock),
// nunca vem do corpo (PA-8).
func (h *Handlers) PayDebt(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body payBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	debt, payment, err := h.svc.Payments.Pay(ctx, id, app.PayInput{
		Amount: body.AmountCents,
		Method: body.Method,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Pagamento registrado.", PayResultDTO{
		Debt:    toDebtDTO(debt),
		Payment: toPaymentDTO(payment),
	})
}
