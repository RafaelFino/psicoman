package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// AppointmentHandlers expõe a tela de pendências e a confirmação de pedidos.
type AppointmentHandlers struct {
	svc   *service.AppointmentService
	audit *service.AuditService
}

// NewAppointmentHandlers cria os handlers de pedidos de agendamento.
func NewAppointmentHandlers(svc *service.AppointmentService, audit *service.AuditService) *AppointmentHandlers {
	return &AppointmentHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de pedidos no grupo autenticado.
func (h *AppointmentHandlers) Register(g *api.Group) {
	g.Handle("GET", "/appointment-requests", h.listPending)
	g.Handle("POST", "/appointment-requests/{id}/confirm", h.confirm)
	g.Handle("POST", "/appointment-requests/{id}/reject", h.reject)
}

func apptView(a *domain.AppointmentRequest) map[string]any {
	return map[string]any{
		"id":          a.ID,
		"patient_id":  a.PatientID,
		"location_id": a.LocationID,
		"slot_start":  a.SlotStart,
		"slot_end":    a.SlotEnd,
		"status":      a.Status,
		"note":        a.Note,
	}
}

func (h *AppointmentHandlers) listPending(w http.ResponseWriter, r *http.Request) {
	rs, err := h.svc.ListPending(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(rs))
	for _, a := range rs {
		views = append(views, apptView(a))
	}
	httpx.Respond(w, r, http.StatusOK, "Pedidos pendentes.", views)
}

type confirmBody struct {
	Modality string `json:"modality"`
}

// confirm confirma o pedido: cria a sessão agendada (checa conflito + evento).
func (h *AppointmentHandlers) confirm(w http.ResponseWriter, r *http.Request) {
	var b confirmBody
	if r.ContentLength > 0 {
		_ = httpx.DecodeJSON(r, &b) // corpo opcional (modalidade)
	}
	sess, err := h.svc.Confirm(r.Context(), r.PathValue("id"), b.Modality)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "appointment_request", r.PathValue("id"),
		map[string]any{"session_id": sess.ID})
	httpx.Respond(w, r, http.StatusCreated, "Pedido confirmado e sessão agendada.", sessionView(sess))
}

func (h *AppointmentHandlers) reject(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Reject(r.Context(), r.PathValue("id")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "appointment_request", r.PathValue("id"), nil)
	httpx.Respond(w, r, http.StatusOK, "Pedido recusado.", nil)
}
