package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/service"
)

// GoogleHandlers expõe o fluxo de autorização OAuth do Google.
type GoogleHandlers struct {
	oauth *google.OAuth
	audit *service.AuditService
}

// NewGoogleHandlers cria os handlers de integração Google.
func NewGoogleHandlers(oauth *google.OAuth, audit *service.AuditService) *GoogleHandlers {
	return &GoogleHandlers{oauth: oauth, audit: audit}
}

// Register instala as rotas de OAuth do Google no grupo autenticado.
func (h *GoogleHandlers) Register(g *api.Group) {
	g.Handle("POST", "/google/authorize", h.authorize)
	g.Handle("POST", "/google/callback", h.callback)
}

// authorize devolve a URL de consentimento do Google.
func (h *GoogleHandlers) authorize(w http.ResponseWriter, r *http.Request) {
	url := h.oauth.AuthURL("psicoman")
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionConfig, "google_oauth", "", nil)
	httpx.Respond(w, r, http.StatusOK, "Autorize o acesso ao Google.", map[string]any{
		"authorize_url": url,
	})
}

type callbackBody struct {
	Code string `json:"code"`
}

// callback troca o authorization code por tokens (refresh token cifrado).
func (h *GoogleHandlers) callback(w http.ResponseWriter, r *http.Request) {
	var b callbackBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	if b.Code == "" {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Informe o campo 'code'."))
		return
	}
	if err := h.oauth.Exchange(r.Context(), b.Code); err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Não foi possível concluir a autorização com o Google."))
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionConfig, "google_oauth", "authorized", nil)
	httpx.Respond(w, r, http.StatusOK, "Google autorizado com sucesso.", nil)
}
