package httpx

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// PropagateActor injeta a identidade resolvida (ActorIdentity) no context do
// request, de onde AuditService.Record a lê via domain.ActorFrom. Deve ser
// ligado DEPOIS de RequirePangolinIdentity (que grava a identidade no store),
// nos grupos /v1/app e /v1/admin. Sem identidade (não deveria ocorrer após o
// gate), o context segue sem ator e o audit_log cai em "sistema".
//
// httpx importa internal/domain (permitido; domain não importa httpx).
func PropagateActor(e *core.RequestEvent) error {
	id := ActorIdentity(e)
	if id != "" {
		e.Request = e.Request.WithContext(domain.WithActor(e.Request.Context(), id))
	}
	return e.Next()
}
