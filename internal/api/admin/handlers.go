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
}

// me devolve o terapeuta autenticado (docs/architecture.md §5.1).
func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	actor := httpx.Actor(r.Context())
	httpx.Respond(w, r, http.StatusOK, "Terapeuta autenticado.", map[string]any{
		"email": actor,
	})
}
