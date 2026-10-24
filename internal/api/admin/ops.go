package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/service"
)

const (
	handlerTimeout  = 5 * time.Second
	backupTimeout   = 2 * time.Minute // backup (zip + DR) é mais lento que I/O comum
	auditDefaultLim = 100
	auditMaxLim     = 500
)

// Handlers agrupa os serviços usados pelos handlers de operação.
type Handlers struct {
	svc *service.Services
}

// Mount registra as rotas /v1/admin/* no grupo informado.
func Mount(g *router.RouterGroup[*core.RequestEvent], svc *service.Services) {
	h := &Handlers{svc: svc}
	g.POST("/backup", h.Backup)
	g.GET("/audit-log", h.AuditLog)
}

// Backup POST /v1/admin/backup — dispara o backup local do PocketBase. O hook
// de DR (internal/drivesync, Fase 7) espelha o snapshot cifrado no Drive quando
// há credenciais; sem elas, é no-op observável (o backup local segue, sem 500).
// Nome vazio => o PocketBase gera um nome padrão (pb_backup_<timestamp>).
func (h *Handlers) Backup(e *core.RequestEvent) error {
	// Backup pode ser mais lento que uma requisição comum (zip + upload de DR);
	// usa um timeout dedicado maior que o handlerTimeout padrão.
	ctx, cancel := context.WithTimeout(e.Request.Context(), backupTimeout)
	defer cancel()
	if err := e.App.CreateBackup(ctx, ""); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondAccepted(e, "Backup realizado.", map[string]any{"status": "ok"})
}

// AuditLog GET /v1/admin/audit-log?limit=N (1..500, default 100).
func (h *Handlers) AuditLog(e *core.RequestEvent) error {
	limit := auditDefaultLim
	if raw := e.Request.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= auditMaxLim {
			limit = n
		} else {
			// Parâmetro de conveniência: clamp para o default com aviso, não 4xx.
			e.App.Logger().Warn("limit de audit-log inválido; usando default",
				"limit", raw, "default", auditDefaultLim)
		}
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	entries, err := h.svc.Audit.List(ctx, limit)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Registros de auditoria.", toAuditLogDTOs(entries))
}

func reqCtx(e *core.RequestEvent) (context.Context, context.CancelFunc) {
	return context.WithTimeout(e.Request.Context(), handlerTimeout)
}
