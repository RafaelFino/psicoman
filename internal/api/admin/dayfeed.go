package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/service"
)

// DayFeedHandlers expõe o feed do dia do terapeuta (patient-timeline R8).
type DayFeedHandlers struct {
	svc *service.DayFeedService
}

// NewDayFeedHandlers cria os handlers do feed do dia.
func NewDayFeedHandlers(svc *service.DayFeedService) *DayFeedHandlers {
	return &DayFeedHandlers{svc: svc}
}

// Register instala a rota do feed do dia no grupo autenticado.
func (h *DayFeedHandlers) Register(g *api.Group) {
	g.Handle("GET", "/day-feed", h.dayFeed)
}

func (h *DayFeedHandlers) dayFeed(w http.ResponseWriter, r *http.Request) {
	window := r.URL.Query().Get("window")
	feed, err := h.svc.Build(r.Context(), window)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Feed do dia.", dayFeedView(feed))
}

func dayFeedView(feed *service.DayFeed) map[string]any {
	eventos := make([]map[string]any, 0, len(feed.Eventos))
	for _, e := range feed.Eventos {
		ev := map[string]any{
			"tipo":         e.Tipo,
			"data":         e.Data,
			"patient_id":   e.PatientID,
			"patient_name": e.PatientName,
			"titulo":       e.Titulo,
			"resumo":       e.Resumo,
			"ref":          map[string]any{"kind": e.Ref.Kind, "id": e.Ref.ID},
		}
		if e.Status != "" {
			ev["status"] = e.Status
		}
		if e.Valor != 0 {
			ev["valor"] = e.Valor
		}
		eventos = append(eventos, ev)
	}
	return map[string]any{
		"window": feed.Window,
		"resumo": map[string]any{
			"counts":    feed.Resumo.Counts,
			"total":     feed.Resumo.Total,
			"recebido":  feed.Resumo.Recebido,
			"a_receber": feed.Resumo.AReceber,
		},
		"eventos": eventos,
	}
}
