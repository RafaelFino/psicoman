package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// TherapistHandlers expõe a gestão do perfil do terapeuta.
type TherapistHandlers struct {
	svc   *service.TherapistService
	audit *service.AuditService
}

// NewTherapistHandlers cria os handlers do perfil.
func NewTherapistHandlers(svc *service.TherapistService, audit *service.AuditService) *TherapistHandlers {
	return &TherapistHandlers{svc: svc, audit: audit}
}

// Register instala as rotas do perfil no grupo autenticado.
func (h *TherapistHandlers) Register(g *api.Group) {
	g.Handle("GET", "/profile", h.get)
	g.Handle("PUT", "/profile", h.save)
	g.Handle("POST", "/profile/photo", h.photo)
	g.Handle("GET", "/profile/links", h.listLinks)
	g.Handle("POST", "/profile/links", h.addLink)
	g.Handle("DELETE", "/profile/links/{id}", h.deleteLink)
}

func profileView(p *domain.TherapistProfile) map[string]any {
	return map[string]any{
		"id":           p.ID,
		"name":         p.Name,
		"crp":          p.CRP,
		"email":        p.Email,
		"contacts":     p.Contacts,
		"bio":          p.Bio,
		"photo_id":     p.PhotoFileID,
		"location_ids": p.LocationIDs,
		"updated_at":   p.UpdatedAt,
	}
}

func linkView(l *domain.TherapistPlatformLink) map[string]any {
	return map[string]any{
		"id":        l.ID,
		"label":     l.Label,
		"url":       l.URL,
		"origin_id": l.OriginID,
	}
}

func (h *TherapistHandlers) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProfile(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Perfil do terapeuta.", profileView(p))
}

type profileBody struct {
	Name        string            `json:"name"`
	CRP         string            `json:"crp"`
	Email       string            `json:"email"`
	Contacts    map[string]string `json:"contacts"`
	Bio         string            `json:"bio"`
	LocationIDs []string          `json:"location_ids"`
}

func (h *TherapistHandlers) save(w http.ResponseWriter, r *http.Request) {
	var b profileBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	p, err := h.svc.SaveProfile(r.Context(), service.ProfileInput{
		Name: b.Name, CRP: b.CRP, Email: b.Email, Contacts: b.Contacts,
		Bio: b.Bio, LocationIDs: b.LocationIDs,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "therapist_profile", p.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Perfil salvo.", profileView(p))
}

func (h *TherapistHandlers) photo(w http.ResponseWriter, r *http.Request) {
	const maxUpload = 10 << 20
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Não foi possível ler a foto enviada."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.RespondError(w, r, httpx.ErrBadRequest("Envie a foto no campo 'file'."))
		return
	}
	defer file.Close()

	p, err := h.svc.SetPhoto(r.Context(), header.Header.Get("Content-Type"), file)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Foto atualizada.", profileView(p))
}

func (h *TherapistHandlers) listLinks(w http.ResponseWriter, r *http.Request) {
	ls, err := h.svc.ListLinks(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ls))
	for _, l := range ls {
		views = append(views, linkView(l))
	}
	httpx.Respond(w, r, http.StatusOK, "Links listados.", views)
}

type linkBody struct {
	Label    string `json:"label"`
	URL      string `json:"url"`
	OriginID string `json:"origin_id"`
}

func (h *TherapistHandlers) addLink(w http.ResponseWriter, r *http.Request) {
	var b linkBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	l, err := h.svc.AddLink(r.Context(), b.Label, b.URL, b.OriginID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Link adicionado.", linkView(l))
}

func (h *TherapistHandlers) deleteLink(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteLink(r.Context(), r.PathValue("id")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Link removido.", nil)
}
