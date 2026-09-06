package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// LocationHandlers expõe o CRUD de locais e disponibilidades.
type LocationHandlers struct {
	svc   *service.LocationService
	audit *service.AuditService
}

// NewLocationHandlers cria os handlers de local.
func NewLocationHandlers(svc *service.LocationService, audit *service.AuditService) *LocationHandlers {
	return &LocationHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de locais no grupo autenticado.
func (h *LocationHandlers) Register(g *api.Group) {
	g.Handle("POST", "/locations", h.create)
	g.Handle("GET", "/locations", h.list)
	g.Handle("GET", "/locations/{id}", h.get)
	g.Handle("PUT", "/locations/{id}", h.update)
	g.Handle("DELETE", "/locations/{id}", h.delete)
	g.Handle("POST", "/locations/{id}/availability", h.addAvailability)
	g.Handle("GET", "/locations/{id}/availability", h.listAvailability)
	g.Handle("DELETE", "/locations/{id}/availability/{avid}", h.deleteAvailability)
}

type locationBody struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Modality   string `json:"modality"`
	CostAmount int64  `json:"cost_amount"`
	CostPeriod string `json:"cost_period"`
}

func locationView(l *domain.Location) map[string]any {
	return map[string]any{
		"id":          l.ID,
		"name":        l.Name,
		"address":     l.Address,
		"modality":    l.Modality,
		"cost_amount": l.CostAmount,
		"cost_period": l.CostPeriod,
		"created_at":  l.CreatedAt,
		"updated_at":  l.UpdatedAt,
	}
}

func availabilityView(a *domain.Availability) map[string]any {
	return map[string]any{
		"id":          a.ID,
		"location_id": a.LocationID,
		"weekday":     a.Weekday,
		"start_time":  a.StartTime,
		"end_time":    a.EndTime,
		"capacity":    a.Capacity,
	}
}

func (h *LocationHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b locationBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	l, err := h.svc.Create(r.Context(), service.LocationInput{
		Name: b.Name, Address: b.Address, Modality: b.Modality,
		CostAmount: b.CostAmount, CostPeriod: b.CostPeriod,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "location", l.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Local cadastrado.", locationView(l))
}

func (h *LocationHandlers) list(w http.ResponseWriter, r *http.Request) {
	ls, err := h.svc.List(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ls))
	for _, l := range ls {
		views = append(views, locationView(l))
	}
	httpx.Respond(w, r, http.StatusOK, "Locais listados.", views)
}

func (h *LocationHandlers) get(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Local encontrado.", locationView(l))
}

func (h *LocationHandlers) update(w http.ResponseWriter, r *http.Request) {
	var b locationBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	l, err := h.svc.Update(r.Context(), r.PathValue("id"), service.LocationInput{
		Name: b.Name, Address: b.Address, Modality: b.Modality,
		CostAmount: b.CostAmount, CostPeriod: b.CostPeriod,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "location", l.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Local atualizado.", locationView(l))
}

func (h *LocationHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDelete, "location", id, nil)
	httpx.Respond(w, r, http.StatusOK, "Local removido.", nil)
}

type availabilityBody struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Capacity  int    `json:"capacity"`
}

func (h *LocationHandlers) addAvailability(w http.ResponseWriter, r *http.Request) {
	var b availabilityBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	a, err := h.svc.AddAvailability(r.Context(), r.PathValue("id"), service.AvailabilityInput{
		Weekday: b.Weekday, StartTime: b.StartTime, EndTime: b.EndTime, Capacity: b.Capacity,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Disponibilidade adicionada.", availabilityView(a))
}

func (h *LocationHandlers) listAvailability(w http.ResponseWriter, r *http.Request) {
	as, err := h.svc.ListAvailability(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(as))
	for _, a := range as {
		views = append(views, availabilityView(a))
	}
	httpx.Respond(w, r, http.StatusOK, "Disponibilidades listadas.", views)
}

func (h *LocationHandlers) deleteAvailability(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAvailability(r.Context(), r.PathValue("avid")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Disponibilidade removida.", nil)
}
