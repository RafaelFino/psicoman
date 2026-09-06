package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// BackupHandlers expõe backup e restore manuais (operações sensíveis).
type BackupHandlers struct {
	svc   *service.BackupService
	audit *service.AuditService
}

// NewBackupHandlers cria os handlers de backup.
func NewBackupHandlers(svc *service.BackupService, audit *service.AuditService) *BackupHandlers {
	return &BackupHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de backup/restore no grupo autenticado.
func (h *BackupHandlers) Register(g *api.Group) {
	g.Handle("POST", "/backup", h.backup)
	g.Handle("POST", "/restore", h.restore)
}

func (h *BackupHandlers) backup(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Backup(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionBackup, "backup", "", nil)
	httpx.Respond(w, r, http.StatusOK, "Backup concluído.", map[string]any{
		"snapshot_file_id": res.SnapshotFileID,
		"ged_uploaded":     res.GEDUploaded,
		"ged_skipped":      res.GEDSkipped,
	})
}

func (h *BackupHandlers) restore(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Restore(r.Context()); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionRestore, "backup", "", nil)
	httpx.Respond(w, r, http.StatusOK, "Restauração concluída. Reinicie a aplicação para aplicar.", nil)
}
