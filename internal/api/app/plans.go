package app

import (
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

type planBody struct {
	Type        string `json:"type"`
	AmountCents int64  `json:"amount_cents"`
	DueDay      int    `json:"due_day"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
}

// ListPatientPlans GET /v1/app/patients/{id}/plans.
func (h *Handlers) ListPatientPlans(e *core.RequestEvent) error {
	pid := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	ps, err := h.svc.Plans.ListByPatient(ctx, pid)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Planos do paciente.", toPlanDTOs(ps))
}

// CreatePatientPlan POST /v1/app/patients/{id}/plans. starts_at obrigatório;
// ends_at opcional (vazio = sem fim).
func (h *Handlers) CreatePatientPlan(e *core.RequestEvent) error {
	pid := e.Request.PathValue("id")
	var body planBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	startsAt, err := clock.Parse(body.StartsAt)
	if err != nil {
		return httpx.FailBadRequest(e, err)
	}
	var endsAt time.Time
	if body.EndsAt != "" {
		endsAt, err = clock.Parse(body.EndsAt)
		if err != nil {
			return httpx.FailBadRequest(e, err)
		}
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Plans.Create(ctx, app.PlanInput{
		PatientID: pid,
		Type:      body.Type,
		Amount:    body.AmountCents,
		DueDay:    body.DueDay,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Plano criado.", toPlanDTO(p))
}

// DeletePatientPlan DELETE /v1/app/plans/{id}.
func (h *Handlers) DeletePatientPlan(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Plans.Delete(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Plano removido.", nil)
}
