package admin

import (
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// SessionHandlers expõe o CRUD e as transições de estado das sessões.
type SessionHandlers struct {
	svc   *service.SessionService
	audit *service.AuditService
}

// NewSessionHandlers cria os handlers de sessão.
func NewSessionHandlers(svc *service.SessionService, audit *service.AuditService) *SessionHandlers {
	return &SessionHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de sessões no grupo autenticado.
func (h *SessionHandlers) Register(g *api.Group) {
	g.Handle("POST", "/sessions", h.create)
	g.Handle("GET", "/sessions", h.list)
	g.Handle("GET", "/sessions/{id}", h.get)
	g.Handle("POST", "/sessions/{id}/schedule", h.schedule)
	g.Handle("POST", "/sessions/{id}/finish", h.finish)
	g.Handle("POST", "/sessions/{id}/cancel", h.cancel)
	g.Handle("POST", "/sessions/{id}/no-show", h.noShow)
}

func sessionView(s *domain.Session) map[string]any {
	return map[string]any{
		"id":            s.ID,
		"patient_id":    s.PatientID,
		"location_id":   s.LocationID,
		"request_id":    s.RequestID,
		"modality":      s.Modality,
		"starts_at":     s.StartsAt,
		"ends_at":       s.EndsAt,
		"status":        s.Status,
		"bill":          s.Bill,
		"consider_cost": s.ConsiderCost,
		"meet_url":      s.MeetURL,
		"created_at":    s.CreatedAt,
		"updated_at":    s.UpdatedAt,
	}
}

type sessionBody struct {
	PatientID  string `json:"patient_id"`
	LocationID string `json:"location_id"`
	Modality   string `json:"modality"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	Status     string `json:"status"`
}

func (h *SessionHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b sessionBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	starts, err1 := time.Parse(time.RFC3339, b.StartsAt)
	ends, err2 := time.Parse(time.RFC3339, b.EndsAt)
	if err1 != nil || err2 != nil {
		httpx.RespondError(w, r, httpx.ErrValidation("Os horários devem estar no formato ISO-8601 (RFC3339)."))
		return
	}
	s, err := h.svc.Create(r.Context(), service.SessionCreateInput{
		PatientID: b.PatientID, LocationID: b.LocationID, Modality: b.Modality,
		StartsAt: starts, EndsAt: ends, Status: b.Status,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "session", s.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Sessão criada.", sessionView(s))
}

func (h *SessionHandlers) list(w http.ResponseWriter, r *http.Request) {
	// Filtro opcional por paciente.
	if pid := r.URL.Query().Get("patient_id"); pid != "" {
		ss, err := h.svc.ListByPatient(r.Context(), pid)
		if err != nil {
			respondServiceError(w, r, err)
			return
		}
		h.respondList(w, r, ss)
		return
	}
	ss, err := h.svc.List(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	h.respondList(w, r, ss)
}

func (h *SessionHandlers) respondList(w http.ResponseWriter, r *http.Request, ss []*domain.Session) {
	views := make([]map[string]any, 0, len(ss))
	for _, s := range ss {
		views = append(views, sessionView(s))
	}
	httpx.Respond(w, r, http.StatusOK, "Sessões listadas.", views)
}

func (h *SessionHandlers) get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Sessão encontrada.", sessionView(s))
}

func (h *SessionHandlers) schedule(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Schedule(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "session", s.ID,
		map[string]any{"status": s.Status})
	httpx.Respond(w, r, http.StatusOK, "Sessão agendada.", sessionView(s))
}

type finishBody struct {
	Bill         bool `json:"bill"`
	ConsiderCost bool `json:"consider_cost"`
}

func (h *SessionHandlers) finish(w http.ResponseWriter, r *http.Request) {
	var b finishBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	s, err := h.svc.Finish(r.Context(), r.PathValue("id"), service.FinishInput{
		Bill: b.Bill, ConsiderCost: b.ConsiderCost,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "session", s.ID,
		map[string]any{"status": s.Status, "bill": s.Bill, "consider_cost": s.ConsiderCost})
	httpx.Respond(w, r, http.StatusOK, "Sessão finalizada.", sessionView(s))
}

func (h *SessionHandlers) cancel(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Cancel(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "session", s.ID,
		map[string]any{"status": s.Status})
	httpx.Respond(w, r, http.StatusOK, "Sessão cancelada.", sessionView(s))
}

func (h *SessionHandlers) noShow(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.MarkNoShow(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "session", s.ID,
		map[string]any{"status": s.Status})
	httpx.Respond(w, r, http.StatusOK, "Sessão marcada como falta.", sessionView(s))
}
