package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
)

type therapistProfileBody struct {
	Name        string            `json:"name"`
	CRP         string            `json:"crp"`
	Email       string            `json:"email"`
	Bio         string            `json:"bio"`
	Contacts    map[string]string `json:"contacts"`
	PhotoFileID string            `json:"photo_file_id"`
	LocationIDs []string          `json:"location_ids"`
}

type therapistLinkBody struct {
	Label    string `json:"label"`
	URL      string `json:"url"`
	OriginID string `json:"origin_id"`
}

// GetTherapist GET /v1/app/therapist — perfil do terapeuta + links.
func (h *Handlers) GetTherapist(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, links, err := h.svc.Therapist.Get(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Perfil do terapeuta.", toTherapistProfileDTO(p, links))
}

// UpsertTherapist PUT /v1/app/therapist — cria ou atualiza o perfil singleton.
func (h *Handlers) UpsertTherapist(e *core.RequestEvent) error {
	var body therapistProfileBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Therapist.Upsert(ctx, app.UpsertProfileInput{
		Name:        body.Name,
		CRP:         body.CRP,
		Email:       body.Email,
		Bio:         body.Bio,
		Contacts:    body.Contacts,
		PhotoFileID: body.PhotoFileID,
		LocationIDs: body.LocationIDs,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	// Relê os links para compor a resposta completa.
	_, links, err := h.svc.Therapist.Get(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Perfil atualizado.", toTherapistProfileDTO(p, links))
}

// AddTherapistLink POST /v1/app/therapist/platform-links — adiciona um link.
func (h *Handlers) AddTherapistLink(e *core.RequestEvent) error {
	var body therapistLinkBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	l, err := h.svc.Therapist.AddLink(ctx, app.AddLinkInput{
		Label:    body.Label,
		URL:      body.URL,
		OriginID: body.OriginID,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Link adicionado.", toTherapistLinkDTO(l))
}

// DeleteTherapistLink DELETE /v1/app/therapist/platform-links/{id}.
func (h *Handlers) DeleteTherapistLink(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Therapist.RemoveLink(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Link removido.", nil)
}
