package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
)

type locationBody struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Modality   string `json:"modality"`
	CostCents  int64  `json:"cost_cents"`
	CostPeriod string `json:"cost_period"`
}

type availabilityBody struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Capacity  int    `json:"capacity"`
}

// ListLocations GET /v1/app/locations.
func (h *Handlers) ListLocations(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	ls, err := h.svc.Locations.List(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Locais listados.", toLocationDTOs(ls))
}

// CreateLocation POST /v1/app/locations.
func (h *Handlers) CreateLocation(e *core.RequestEvent) error {
	var body locationBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	l, err := h.svc.Locations.Create(ctx, app.LocationInput{
		Name:       body.Name,
		Address:    body.Address,
		Modality:   body.Modality,
		CostAmount: body.CostCents,
		CostPeriod: body.CostPeriod,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Local criado.", toLocationDTO(l))
}

// GetLocation GET /v1/app/locations/{id}.
func (h *Handlers) GetLocation(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	l, err := h.svc.Locations.Get(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Local encontrado.", toLocationDTO(l))
}

// UpdateLocation PATCH /v1/app/locations/{id}.
func (h *Handlers) UpdateLocation(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body locationBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	l, err := h.svc.Locations.Update(ctx, id, app.LocationInput{
		Name:       body.Name,
		Address:    body.Address,
		Modality:   body.Modality,
		CostAmount: body.CostCents,
		CostPeriod: body.CostPeriod,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Local atualizado.", toLocationDTO(l))
}

// DeleteLocation DELETE /v1/app/locations/{id}.
func (h *Handlers) DeleteLocation(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Locations.Delete(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Local removido.", nil)
}

// ListAvailability GET /v1/app/locations/{id}/availability.
func (h *Handlers) ListAvailability(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	as, err := h.svc.Locations.ListAvailability(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Disponibilidades do local.", toAvailabilityDTOs(as))
}

// AddAvailability POST /v1/app/locations/{id}/availability.
func (h *Handlers) AddAvailability(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body availabilityBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	a, err := h.svc.Locations.AddAvailability(ctx, id, app.AvailabilityInput{
		Weekday:   body.Weekday,
		StartTime: body.StartTime,
		EndTime:   body.EndTime,
		Capacity:  body.Capacity,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Disponibilidade adicionada.", toAvailabilityDTO(a))
}

// DeleteAvailability DELETE /v1/app/locations/availability/{id}.
func (h *Handlers) DeleteAvailability(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Locations.DeleteAvailability(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Disponibilidade removida.", nil)
}
