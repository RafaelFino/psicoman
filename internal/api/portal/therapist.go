package portal

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
)

// Therapist GET /v1/portal/therapist — dados PÚBLICOS do perfil do terapeuta
// (nome/foto/bio, via PublicView). Nunca expõe CRP/contatos/email. Fora do gate
// de aprovação (o paciente vê quem o atende antes mesmo de ser aprovado).
func (h *Handlers) Therapist(e *core.RequestEvent) error {
	// Fora do gate de APROVAÇÃO, mas exige sessão válida de paciente (pendente
	// ou aprovado): o perfil não é exposto a visitantes anônimos.
	if httpx.PortalAuthRecordID(e) == "" {
		return httpx.RespondError(e, http.StatusUnauthorized, "nao_autenticado", "Autenticação necessária.", nil)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, _, err := h.svc.Therapist.Get(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Perfil do terapeuta.", p.PublicView())
}
