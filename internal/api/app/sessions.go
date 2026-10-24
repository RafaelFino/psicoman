package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

type sessionCreateBody struct {
	PatientID  string `json:"patient_id"`
	LocationID string `json:"location_id"`
	Modality   string `json:"modality"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	Status     string `json:"status"`
}

type sessionUpdateBody struct {
	LocationID string `json:"location_id"`
	Modality   string `json:"modality"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
}

type cancelBody struct {
	Reason string `json:"reason"`
}

type finishBody struct {
	Bill         *bool `json:"bill"`
	ConsiderCost bool  `json:"consider_cost"`
}

// CreateSession POST /v1/app/sessions.
func (h *Handlers) CreateSession(e *core.RequestEvent) error {
	var body sessionCreateBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	startsAt, err := clock.Parse(body.StartsAt)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	endsAt, err := clock.Parse(body.EndsAt)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Create(ctx, app.SessionCreateInput{
		PatientID:  body.PatientID,
		LocationID: body.LocationID,
		Modality:   body.Modality,
		StartsAt:   startsAt,
		EndsAt:     endsAt,
		Status:     body.Status,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Sessão criada.", toSessionDTO(s))
}

// GetSession GET /v1/app/sessions/{id}.
func (h *Handlers) GetSession(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Get(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessão encontrada.", toSessionDTO(s))
}

// UpdateSession PATCH /v1/app/sessions/{id} — full-replace dos campos editáveis.
func (h *Handlers) UpdateSession(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body sessionUpdateBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	startsAt, err := clock.Parse(body.StartsAt)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	endsAt, err := clock.Parse(body.EndsAt)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Update(ctx, id, app.SessionUpdateInput{
		LocationID: body.LocationID,
		Modality:   body.Modality,
		StartsAt:   startsAt,
		EndsAt:     endsAt,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessão atualizada.", toSessionDTO(s))
}

// ConfirmSession POST /v1/app/sessions/{id}/confirm.
func (h *Handlers) ConfirmSession(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Confirm(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessão confirmada.", toSessionDTO(s))
}

// CancelSession POST /v1/app/sessions/{id}/cancel.
func (h *Handlers) CancelSession(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body cancelBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	// TODO Fase 5: extrair o autor do AuthAppStub; por ora é a terapeuta.
	by := scheduling.CancelByTherapist
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Cancel(ctx, id, by, body.Reason)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessão cancelada.", toSessionDTO(s))
}

// FinishSession POST /v1/app/sessions/{id}/finish.
func (h *Handlers) FinishSession(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body finishBody
	// Corpo vazio é válido (defaults do serviço); ignora erro de corpo ausente.
	_ = e.BindBody(&body)
	ctx, cancel := reqCtx(e)
	defer cancel()
	s, err := h.svc.Sessions.Finish(ctx, id, app.FinishInput{
		Bill:         body.Bill,
		ConsiderCost: body.ConsiderCost,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessão finalizada.", toSessionDTO(s))
}
