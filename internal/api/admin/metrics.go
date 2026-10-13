package admin

import (
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// MetricsHandlers expõe as métricas de sessão por paciente e o relatório por
// local (patient-timeline R6.4–R6.5, R7).
type MetricsHandlers struct {
	svc *service.SessionMetricsService
}

// NewMetricsHandlers cria os handlers de métricas.
func NewMetricsHandlers(svc *service.SessionMetricsService) *MetricsHandlers {
	return &MetricsHandlers{svc: svc}
}

// Register instala as rotas de métricas no grupo autenticado.
func (h *MetricsHandlers) Register(g *api.Group) {
	g.Handle("GET", "/patients/{id}/session-metrics", h.patientMetrics)
	g.Handle("GET", "/locations/{id}/session-report", h.locationReport)
}

// parseRange lê from/to (RFC3339) opcionais da query. Devolve zero quando ausentes.
func parseRange(r *http.Request) (from, to time.Time, err error) {
	q := r.URL.Query()
	if v := q.Get("from"); v != "" {
		from, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return
		}
	}
	if v := q.Get("to"); v != "" {
		to, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return
		}
	}
	return
}

func (h *MetricsHandlers) patientMetrics(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrValidation("from/to devem estar em ISO-8601 (RFC3339)."))
		return
	}
	m, err := h.svc.PatientSessionMetrics(r.Context(), r.PathValue("id"), from, to)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	if !from.IsZero() {
		m.From = clock.Format(from)
	}
	if !to.IsZero() {
		m.To = clock.Format(to)
	}
	httpx.Respond(w, r, http.StatusOK, "Métricas de sessão do paciente.", metricsView(m))
}

func metricsView(m *service.PatientMetrics) map[string]any {
	return map[string]any{
		"counts":              m.Counts,
		"total":               m.Total,
		"faturado_realizadas": m.FaturadoReal,
		"taxa_comparecimento": m.TaxaComparecimento,
		"taxa_cancelamento":   m.TaxaCancelamento,
		"from":                m.From,
		"to":                  m.To,
	}
}

func (h *MetricsHandlers) locationReport(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrValidation("from/to devem estar em ISO-8601 (RFC3339)."))
		return
	}
	// Default: mês atual (primeiro ao primeiro dia do mês seguinte), fuso local.
	if from.IsZero() && to.IsZero() {
		now := clock.Now()
		loc := clock.Location()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		to = from.AddDate(0, 1, 0)
	}
	rep, err := h.svc.LocationSessionReport(r.Context(), r.PathValue("id"), from, to)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	rep.From = clock.Format(from)
	rep.To = clock.Format(to)
	httpx.Respond(w, r, http.StatusOK, "Relatório de sessões por local.", locationReportView(rep))
}

func locationReportView(rep *service.LocationReport) map[string]any {
	return map[string]any{
		"location_id":         rep.LocationID,
		"location_name":       rep.LocationName,
		"counts":              rep.Counts,
		"total":               rep.Total,
		"faturado_realizadas": rep.FaturadoReal,
		"from":                rep.From,
		"to":                  rep.To,
	}
}
