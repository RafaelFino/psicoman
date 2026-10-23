package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
)

// ReportFinancial GET /v1/app/reports/financial?de=YYYY-MM-DD&ate=YYYY-MM-DD.
func (h *Handlers) ReportFinancial(e *core.RequestEvent) error {
	de, ate, err := httpx.ParseDateRange(e.Request.URL.Query().Get("de"), e.Request.URL.Query().Get("ate"))
	if err != nil {
		return httpx.RespondError(e, 400, "requisicao_invalida",
			"Parâmetros de período inválidos (use ?de=&ate= no formato AAAA-MM-DD).", nil)
	}
	if de.After(ate) {
		return httpx.Fail(e, app.NewValidation("Data inicial não pode ser posterior à final."))
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	sum, err := h.svc.Reports.Financial(ctx, de, ate)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Resumo financeiro do período.", toFinancialSummaryDTO(sum))
}

// ReportROI GET /v1/app/reports/roi?de=YYYY-MM-DD&ate=YYYY-MM-DD.
func (h *Handlers) ReportROI(e *core.RequestEvent) error {
	de, ate, err := httpx.ParseDateRange(e.Request.URL.Query().Get("de"), e.Request.URL.Query().Get("ate"))
	if err != nil {
		return httpx.RespondError(e, 400, "requisicao_invalida",
			"Parâmetros de período inválidos (use ?de=&ate= no formato AAAA-MM-DD).", nil)
	}
	if de.After(ate) {
		return httpx.Fail(e, app.NewValidation("Data inicial não pode ser posterior à final."))
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	rows, err := h.svc.Reports.ROI(ctx, de, ate)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "ROI por canal no período.", toROIRowDTOs(rows))
}
