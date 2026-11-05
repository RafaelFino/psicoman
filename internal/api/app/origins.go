package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
)

type originBody struct {
	Name string `json:"name"`
}

// ListOrigins GET /v1/app/origins.
func (h *Handlers) ListOrigins(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	os, err := h.svc.Origins.List(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Origens listadas.", toOriginDTOs(os))
}

// CreateOrigin POST /v1/app/origins.
func (h *Handlers) CreateOrigin(e *core.RequestEvent) error {
	var body originBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	o, err := h.svc.Origins.Create(ctx, body.Name)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Origem criada.", toOriginDTO(o))
}

// UpdateOrigin PATCH /v1/app/origins/{id}.
func (h *Handlers) UpdateOrigin(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body originBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	o, err := h.svc.Origins.Update(ctx, id, body.Name)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Origem atualizada.", toOriginDTO(o))
}

// DeleteOrigin DELETE /v1/app/origins/{id}.
func (h *Handlers) DeleteOrigin(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Origins.Delete(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Origem removida.", nil)
}
