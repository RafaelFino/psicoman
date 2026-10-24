package app

import (
	"net/http"

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

// GenerateInvoice POST /v1/app/debts/{id}/invoice: gera (ou reaproveita) o PDF
// de cobrança do débito, armazena no GED e vincula ao débito. Devolve o
// metadado do arquivo. Sem envio por email (D9b): o canal é a terapeuta.
func (h *Handlers) GenerateInvoice(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	f, err := h.svc.Invoice.GeneratePDF(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "PDF de cobrança gerado.", toInvoiceDTO(f))
}

// DownloadInvoice GET /v1/app/debts/{id}/invoice.pdf: devolve os BYTES do PDF
// de cobrança (gera on-demand se ainda não existir). Resposta binária fora do
// envelope JSON (download direto, application/pdf).
func (h *Handlers) DownloadInvoice(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	data, err := h.svc.Invoice.GetPDF(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	e.Response.Header().Set("Content-Disposition", `attachment; filename="cobranca-`+id+`.pdf"`)
	return e.Blob(http.StatusOK, "application/pdf", data)
}
