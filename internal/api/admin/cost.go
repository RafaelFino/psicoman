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

// CostHandlers expõe o CRUD de custos e os relatórios de custo/ROI.
type CostHandlers struct {
	svc     *service.CostService
	reports *service.ReportService
	audit   *service.AuditService
}

// NewCostHandlers cria os handlers de custo.
func NewCostHandlers(svc *service.CostService, reports *service.ReportService, audit *service.AuditService) *CostHandlers {
	return &CostHandlers{svc: svc, reports: reports, audit: audit}
}

// Register instala as rotas de custo e relatórios no grupo autenticado.
func (h *CostHandlers) Register(g *api.Group) {
	g.Handle("POST", "/costs/categories", h.createCategory)
	g.Handle("GET", "/costs/categories", h.listCategories)
	g.Handle("POST", "/costs/items", h.createItem)
	g.Handle("GET", "/costs/items", h.listItems)
	g.Handle("DELETE", "/costs/items/{id}", h.deleteItem)
	g.Handle("GET", "/sessions/{id}/cost", h.sessionCost)

	g.Handle("GET", "/reports/financial", h.reportFinancial)
	g.Handle("GET", "/reports/costs", h.reportCosts)
	g.Handle("GET", "/reports/roi", h.reportROI)
}

type categoryBody struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (h *CostHandlers) createCategory(w http.ResponseWriter, r *http.Request) {
	var b categoryBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), b.Kind, b.Name)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Categoria criada.", map[string]any{
		"id": c.ID, "kind": c.Kind, "name": c.Name,
	})
}

func (h *CostHandlers) listCategories(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.ListCategories(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		views = append(views, map[string]any{"id": c.ID, "kind": c.Kind, "name": c.Name})
	}
	httpx.Respond(w, r, http.StatusOK, "Categorias listadas.", views)
}

type itemBody struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	Amount     int64  `json:"amount"`
	Period     string `json:"period"`
	OriginID   string `json:"origin_id"`
	LocationID string `json:"location_id"`
}

func costItemView(i *domain.CostItem) map[string]any {
	return map[string]any{
		"id":          i.ID,
		"category_id": i.CategoryID,
		"name":        i.Name,
		"amount":      i.Amount,
		"period":      i.Period,
		"origin_id":   i.OriginID,
		"location_id": i.LocationID,
	}
}

func (h *CostHandlers) createItem(w http.ResponseWriter, r *http.Request) {
	var b itemBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	i, err := h.svc.CreateItem(r.Context(), service.CostItemInput{
		CategoryID: b.CategoryID, Name: b.Name, Amount: b.Amount, Period: b.Period,
		OriginID: b.OriginID, LocationID: b.LocationID,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Item de custo criado.", costItemView(i))
}

func (h *CostHandlers) listItems(w http.ResponseWriter, r *http.Request) {
	is, err := h.svc.ListItems(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(is))
	for _, i := range is {
		views = append(views, costItemView(i))
	}
	httpx.Respond(w, r, http.StatusOK, "Itens de custo listados.", views)
}

func (h *CostHandlers) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteItem(r.Context(), r.PathValue("id")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Item de custo removido.", nil)
}

func (h *CostHandlers) sessionCost(w http.ResponseWriter, r *http.Request) {
	sc, err := h.svc.GetSessionCost(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Custo da sessão.", map[string]any{
		"session_id":    sc.SessionID,
		"amount":        sc.Amount,
		"method":        sc.Method,
		"base_snapshot": sc.BaseSnapshot,
	})
}

// parsePeriod lê os parâmetros de query `from`/`to` (ISO-8601). Default: mês atual.
func parsePeriod(r *http.Request) (time.Time, time.Time) {
	now := clock.Now()
	de := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, clock.Location())
	ate := de.AddDate(0, 1, 0)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			de = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			ate = t
		}
	}
	return de, ate
}

func (h *CostHandlers) reportFinancial(w http.ResponseWriter, r *http.Request) {
	de, ate := parsePeriod(r)
	sum, err := h.reports.Financial(r.Context(), de, ate)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Relatório financeiro.", map[string]any{
		"generated": sum.Generated,
		"open":      sum.Open,
		"received":  sum.Received,
		"overdue":   sum.Overdue,
	})
}

func (h *CostHandlers) reportCosts(w http.ResponseWriter, r *http.Request) {
	de, ate := parsePeriod(r)
	rep, err := h.reports.Costs(r.Context(), de, ate)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Relatório de custos.", map[string]any{
		"by_kind":    rep.ByKind,
		"by_patient": rep.ByPatient,
	})
}

func (h *CostHandlers) reportROI(w http.ResponseWriter, r *http.Request) {
	de, ate := parsePeriod(r)
	rows, err := h.reports.ROI(r.Context(), de, ate)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Relatório de ROI por canal.", rows)
}
