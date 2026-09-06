package admin

import (
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PlanHandlers expõe o CRUD de planos por paciente.
type PlanHandlers struct {
	svc   *service.PlanService
	audit *service.AuditService
}

// NewPlanHandlers cria os handlers de plano.
func NewPlanHandlers(svc *service.PlanService, audit *service.AuditService) *PlanHandlers {
	return &PlanHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de planos no grupo autenticado.
func (h *PlanHandlers) Register(g *api.Group) {
	g.Handle("POST", "/plans", h.create)
	g.Handle("GET", "/plans", h.list)
	g.Handle("GET", "/plans/{id}", h.get)
	g.Handle("DELETE", "/plans/{id}", h.delete)
}

func planView(p *domain.Plan) map[string]any {
	view := map[string]any{
		"id":         p.ID,
		"patient_id": p.PatientID,
		"type":       p.Type,
		"amount":     p.Amount,
		"starts_at":  p.StartsAt,
		"created_at": p.CreatedAt,
	}
	if !p.EndsAt.IsZero() {
		view["ends_at"] = p.EndsAt
	}
	return view
}

type planBody struct {
	PatientID string `json:"patient_id"`
	Type      string `json:"type"`
	Amount    int64  `json:"amount"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
}

func (h *PlanHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b planBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	in := service.PlanInput{PatientID: b.PatientID, Type: b.Type, Amount: b.Amount}
	if b.StartsAt != "" {
		if t, err := time.Parse(time.RFC3339, b.StartsAt); err == nil {
			in.StartsAt = t
		} else {
			httpx.RespondError(w, r, httpx.ErrValidation("starts_at deve estar em ISO-8601."))
			return
		}
	}
	if b.EndsAt != "" {
		if t, err := time.Parse(time.RFC3339, b.EndsAt); err == nil {
			in.EndsAt = t
		}
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "plan", p.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Plano criado.", planView(p))
}

func (h *PlanHandlers) list(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("patient_id")
	if pid == "" {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Informe o parâmetro patient_id."))
		return
	}
	ps, err := h.svc.ListByPatient(r.Context(), pid)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		views = append(views, planView(p))
	}
	httpx.Respond(w, r, http.StatusOK, "Planos listados.", views)
}

func (h *PlanHandlers) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Plano encontrado.", planView(p))
}

func (h *PlanHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDelete, "plan", id, nil)
	httpx.Respond(w, r, http.StatusOK, "Plano removido.", nil)
}
