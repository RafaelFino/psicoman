package app

import (
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// CostCategoryDTO é a projeção de uma categoria de custo.
type CostCategoryDTO struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// CostItemDTO é a projeção de um item de custo. Dinheiro em centavos.
type CostItemDTO struct {
	ID          string `json:"id"`
	CategoryID  string `json:"category_id,omitempty"`
	Name        string `json:"name"`
	AmountCents int64  `json:"amount_cents"`
	Period      string `json:"period"`
	OriginID    string `json:"origin_id,omitempty"`
	LocationID  string `json:"location_id,omitempty"`
}

// SessionCostDTO é o custo atribuído a uma sessão. Dinheiro em centavos.
type SessionCostDTO struct {
	SessionID   string `json:"session_id"`
	AmountCents int64  `json:"amount_cents"`
	Method      string `json:"method"`
}

// CostReportDTO agrega custos do período, em centavos.
type CostReportDTO struct {
	ByKindCents    map[string]int64 `json:"by_kind_cents"`
	ByPatientCents map[string]int64 `json:"by_patient_cents"`
}

// LocationReportDTO é o relatório de sessões de um local no período. O
// faturado vem em centavos dentro do agregado.
type LocationReportDTO struct {
	LocationID   string `json:"location_id"`
	LocationName string `json:"location_name"`
	From         string `json:"from"`
	To           string `json:"to"`
	Report       any    `json:"report"`
}

type costCategoryBody struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type costItemBody struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	AmountCents int64  `json:"amount_cents"`
	Period      string `json:"period"`
	OriginID    string `json:"origin_id"`
	LocationID  string `json:"location_id"`
}

func toCostItemDTO(i *costs.CostItem) CostItemDTO {
	return CostItemDTO{
		ID: i.ID, CategoryID: i.CategoryID, Name: i.Name, AmountCents: i.Amount,
		Period: i.Period, OriginID: i.OriginID, LocationID: i.LocationID,
	}
}

// ListCostCategories GET /v1/app/cost-categories.
func (h *Handlers) ListCostCategories(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	cs, err := h.svc.Costs.ListCategories(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	out := make([]CostCategoryDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, CostCategoryDTO{ID: c.ID, Kind: c.Kind, Name: c.Name})
	}
	return httpx.RespondOK(e, "Categorias de custo.", out)
}

// CreateCostCategory POST /v1/app/cost-categories.
func (h *Handlers) CreateCostCategory(e *core.RequestEvent) error {
	var body costCategoryBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	c, err := h.svc.Costs.CreateCategory(ctx, body.Kind, body.Name)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Categoria de custo criada.", CostCategoryDTO{ID: c.ID, Kind: c.Kind, Name: c.Name})
}

// ListCostItems GET /v1/app/cost-items.
func (h *Handlers) ListCostItems(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	is, err := h.svc.Costs.ListItems(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	out := make([]CostItemDTO, 0, len(is))
	for _, i := range is {
		out = append(out, toCostItemDTO(i))
	}
	return httpx.RespondOK(e, "Itens de custo.", out)
}

// CreateCostItem POST /v1/app/cost-items.
func (h *Handlers) CreateCostItem(e *core.RequestEvent) error {
	var body costItemBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	i, err := h.svc.Costs.CreateItem(ctx, app.CostItemInput{
		CategoryID: body.CategoryID, Name: body.Name, Amount: body.AmountCents,
		Period: body.Period, OriginID: body.OriginID, LocationID: body.LocationID,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Item de custo criado.", toCostItemDTO(i))
}

// DeleteCostItem DELETE /v1/app/cost-items/{id}.
func (h *Handlers) DeleteCostItem(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Costs.DeleteItem(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Item de custo removido.", nil)
}

// GetSessionCost GET /v1/app/sessions/{id}/cost.
func (h *Handlers) GetSessionCost(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	sc, err := h.svc.Costs.GetSessionCost(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Custo da sessão.", SessionCostDTO{
		SessionID: sc.SessionID, AmountCents: sc.Amount, Method: sc.Method,
	})
}

// ReportCosts GET /v1/app/reports/costs?de=&ate= (AAAA-MM-DD, ate inclusivo).
func (h *Handlers) ReportCosts(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	de, ate, err := httpx.ParseDateRange(q.Get("de"), q.Get("ate"))
	if err != nil {
		return httpx.RespondError(e, 400, "requisicao_invalida",
			"Parâmetros de período inválidos (use ?de=&ate= no formato AAAA-MM-DD).", nil)
	}
	if de.After(ate) {
		return httpx.Fail(e, app.NewValidation("Data inicial não pode ser posterior à final."))
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	rep, err := h.svc.Reports.Costs(ctx, de, ate)
	if err != nil {
		return httpx.Fail(e, err)
	}
	byKind, byPatient := rep.ByKind, rep.ByPatient
	if byKind == nil {
		byKind = map[string]int64{}
	}
	if byPatient == nil {
		byPatient = map[string]int64{}
	}
	return httpx.RespondOK(e, "Relatório de custos do período.", CostReportDTO{
		ByKindCents: byKind, ByPatientCents: byPatient,
	})
}

// LocationSessionReport GET /v1/app/locations/{id}/session-report?de=&ate=.
// Sem de/ate, usa o mês corrente (America/Sao_Paulo). O recorte é [de, ate+1d).
func (h *Handlers) LocationSessionReport(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	q := e.Request.URL.Query()
	var from, to time.Time
	if q.Get("de") == "" && q.Get("ate") == "" {
		now := clock.Now()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, clock.Location())
		to = from.AddDate(0, 1, 0)
	} else {
		var perr error
		from, to, perr = parseOptionalRange(q.Get("de"), q.Get("ate"))
		if perr != nil {
			return httpx.FailBadRequest(e, perr)
		}
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	rep, err := h.svc.Metrics.LocationSessionReport(ctx, id, from, to)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Relatório de sessões do local.", LocationReportDTO{
		LocationID:   rep.LocationID,
		LocationName: rep.LocationName,
		From:         httpx.FmtTime(from),
		To:           httpx.FmtTime(to),
		Report:       rep.SessionAggregate,
	})
}
