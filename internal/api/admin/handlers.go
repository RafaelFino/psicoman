package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/service"
)

// Handlers agrupa as dependências dos handlers administrativos.
type Handlers struct {
	cfg   *config.Config
	audit *service.AuditService
}

// NewHandlers cria o conjunto de handlers do admin.
func NewHandlers(cfg *config.Config, audit *service.AuditService) *Handlers {
	return &Handlers{cfg: cfg, audit: audit}
}

// Register instala as rotas administrativas no grupo /v1/admin (já autenticado).
func (h *Handlers) Register(g *api.Group) {
	g.Handle("GET", "/me", h.me)
	g.Handle("GET", "/audit-log", h.auditLog)
}

// auditLog devolve as entradas de auditoria mais recentes (operações sensíveis).
func (h *Handlers) auditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.audit.List(r.Context(), 200)
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrInternal(err))
		return
	}
	views := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		views = append(views, map[string]any{
			"actor":      e.Actor,
			"action":     e.Action,
			"entity":     e.Entity,
			"entity_id":  e.EntityID,
			"created_at": e.CreatedAt,
		})
	}
	httpx.Respond(w, r, http.StatusOK, "Registro de auditoria.", views)
}

// me devolve o terapeuta autenticado (docs/architecture.md §5.1).
func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	actor := httpx.Actor(r.Context())
	httpx.Respond(w, r, http.StatusOK, "Terapeuta autenticado.", map[string]any{
		"email": actor,
	})
}
