package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// AppointmentRequestDTO é a projeção de um pedido de agendamento para o terapeuta.
type AppointmentRequestDTO struct {
	ID         string `json:"id"`
	PatientID  string `json:"patient_id"`
	LocationID string `json:"location_id,omitempty"`
	SlotStart  string `json:"slot_start"`
	SlotEnd    string `json:"slot_end"`
	Status     string `json:"status"`
	Note       string `json:"note,omitempty"`
}

func toAppointmentRequestDTO(a *scheduling.AppointmentRequest) AppointmentRequestDTO {
	return AppointmentRequestDTO{
		ID: a.ID, PatientID: a.PatientID, LocationID: a.LocationID,
		SlotStart: httpx.FmtTime(a.SlotStart), SlotEnd: httpx.FmtTime(a.SlotEnd),
		Status: a.Status, Note: a.Note,
	}
}

// ListAppointmentRequests GET /v1/app/appointment-requests — fila de pendentes.
func (h *Handlers) ListAppointmentRequests(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	as, err := h.svc.Appts.ListPending(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	out := make([]AppointmentRequestDTO, 0, len(as))
	for _, a := range as {
		out = append(out, toAppointmentRequestDTO(a))
	}
	return httpx.RespondOK(e, "Pedidos de agendamento pendentes.", out)
}

// ConfirmAppointmentRequest POST /v1/app/appointment-requests/{id}/confirm —
// cria a sessão agendada (conflito de agenda → 409; o pedido segue pendente).
func (h *Handlers) ConfirmAppointmentRequest(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Appts.Confirm(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Pedido confirmado e sessão agendada.", toSessionDTO(s))
}

// RefuseAppointmentRequest POST /v1/app/appointment-requests/{id}/refuse.
func (h *Handlers) RefuseAppointmentRequest(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Appts.Refuse(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Pedido recusado.", nil)
}
