package portal

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// PortalAppointmentDTO é o pedido de agendamento visto pelo próprio paciente.
type PortalAppointmentDTO struct {
	ID         string `json:"id"`
	LocationID string `json:"location_id,omitempty"`
	SlotStart  string `json:"slot_start"`
	SlotEnd    string `json:"slot_end"`
	Status     string `json:"status"`
	Note       string `json:"note,omitempty"`
}

func toAppointmentDTO(a *scheduling.AppointmentRequest) PortalAppointmentDTO {
	return PortalAppointmentDTO{
		ID: a.ID, LocationID: a.LocationID,
		SlotStart: httpx.FmtTime(a.SlotStart), SlotEnd: httpx.FmtTime(a.SlotEnd),
		Status: a.Status, Note: a.Note,
	}
}

type appointmentBody struct {
	LocationID string `json:"location_id"`
	SlotStart  string `json:"slot_start"`
	SlotEnd    string `json:"slot_end"`
	Note       string `json:"note"`
}

// maxNoteLen limita a observação livre do paciente.
const maxNoteLen = 500

// CreateAppointment POST /v1/portal/appointment-requests — o paciente APROVADO
// (gate) pede um horário. A identidade vem da sessão, nunca do corpo.
func (h *Handlers) CreateAppointment(e *core.RequestEvent) error {
	var body appointmentBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	start, err := clock.Parse(body.SlotStart)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	end, err := clock.Parse(body.SlotEnd)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	if len([]rune(body.Note)) > maxNoteLen {
		return httpx.Fail(e, app.NewValidation("A observação deve ter no máximo 500 caracteres."))
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.resolvePatient(e, ctx)
	if err != nil {
		return err
	}
	a, err := h.svc.Appts.Request(ctx, p.ID, app.AppointmentInput{
		LocationID: body.LocationID, SlotStart: start, SlotEnd: end, Note: body.Note,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Pedido de agendamento enviado.", toAppointmentDTO(a))
}

// ListAppointments GET /v1/portal/appointment-requests — só os pedidos do
// próprio paciente.
func (h *Handlers) ListAppointments(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.resolvePatient(e, ctx)
	if err != nil {
		return err
	}
	as, err := h.svc.Appts.ListByPatient(ctx, p.ID)
	if err != nil {
		return httpx.Fail(e, err)
	}
	out := make([]PortalAppointmentDTO, 0, len(as))
	for _, a := range as {
		out = append(out, toAppointmentDTO(a))
	}
	return httpx.RespondOK(e, "Seus pedidos de agendamento.", out)
}
