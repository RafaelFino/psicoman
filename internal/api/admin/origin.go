package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// OriginHandlers expõe o CRUD de origens (canais de aquisição).
type OriginHandlers struct {
	svc   *service.OriginService
	audit *service.AuditService
}

// NewOriginHandlers cria os handlers de origem.
func NewOriginHandlers(svc *service.OriginService, audit *service.AuditService) *OriginHandlers {
	return &OriginHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de origens no grupo autenticado.
func (h *OriginHandlers) Register(g *api.Group) {
	g.Handle("POST", "/origins", h.create)
	g.Handle("GET", "/origins", h.list)
	g.Handle("PUT", "/origins/{id}", h.update)
	g.Handle("DELETE", "/origins/{id}", h.delete)
}

type originBody struct {
	Name string `json:"name"`
}

func originView(o *domain.Origin) map[string]any {
	return map[string]any{
		"id":         o.ID,
		"name":       o.Name,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
	}
}

func (h *OriginHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b originBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	o, err := h.svc.Create(r.Context(), b.Name)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "origin", o.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Origem cadastrada.", originView(o))
}

func (h *OriginHandlers) list(w http.ResponseWriter, r *http.Request) {
	os, err := h.svc.List(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(os))
	for _, o := range os {
		views = append(views, originView(o))
	}
	httpx.Respond(w, r, http.StatusOK, "Origens listadas.", views)
}

func (h *OriginHandlers) update(w http.ResponseWriter, r *http.Request) {
	var b originBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	o, err := h.svc.Update(r.Context(), r.PathValue("id"), b.Name)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "origin", o.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Origem atualizada.", originView(o))
}

func (h *OriginHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDelete, "origin", id, nil)
	httpx.Respond(w, r, http.StatusOK, "Origem removida.", nil)
}
