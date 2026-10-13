package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/service"
)

// HistoryHandlers expõe a timeline (história completa) do paciente.
type HistoryHandlers struct {
	svc *service.HistoryService
}

// NewHistoryHandlers cria os handlers de histórico.
func NewHistoryHandlers(svc *service.HistoryService) *HistoryHandlers {
	return &HistoryHandlers{svc: svc}
}

// Register instala a rota de histórico no grupo autenticado.
func (h *HistoryHandlers) Register(g *api.Group) {
	g.Handle("GET", "/patients/{id}/history", h.history)
}

func historyView(items []service.HistoryItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"tipo":      it.Tipo,
			"categoria": it.Categoria,
			"data":      it.Data,
			"titulo":    it.Titulo,
			"resumo":    it.Resumo,
			"ref":       map[string]any{"kind": it.Ref.Kind, "id": it.Ref.ID},
		})
	}
	return out
}

func (h *HistoryHandlers) history(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.PatientHistory(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "História do paciente.", historyView(items))
}
