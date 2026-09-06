package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// ProntuarioHandlers expõe anamnese, notas e templates (dados sensíveis).
type ProntuarioHandlers struct {
	svc   *service.ProntuarioService
	audit *service.AuditService
}

// NewProntuarioHandlers cria os handlers de prontuário.
func NewProntuarioHandlers(svc *service.ProntuarioService, audit *service.AuditService) *ProntuarioHandlers {
	return &ProntuarioHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de prontuário no grupo autenticado.
func (h *ProntuarioHandlers) Register(g *api.Group) {
	g.Handle("GET", "/patients/{id}/anamnesis", h.getAnamnesis)
	g.Handle("PUT", "/patients/{id}/anamnesis", h.saveAnamnesis)
	g.Handle("GET", "/patients/{id}/notes", h.listNotes)
	g.Handle("POST", "/patients/{id}/notes", h.addNote)
	g.Handle("DELETE", "/notes/{nid}", h.deleteNote)

	g.Handle("GET", "/templates", h.listTemplates)
	g.Handle("POST", "/templates", h.createTemplate)
	g.Handle("GET", "/templates/{id}", h.getTemplate)
	g.Handle("GET", "/templates/{id}/render", h.renderTemplate)
	g.Handle("POST", "/templates/{id}/send", h.sendTemplate)
	g.Handle("DELETE", "/templates/{id}", h.deleteTemplate)
}

func (h *ProntuarioHandlers) getAnamnesis(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	a, err := h.svc.GetAnamnesis(r.Context(), pid)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionRead, "anamnesis", pid, nil)
	httpx.Respond(w, r, http.StatusOK, "Anamnese.", map[string]any{
		"patient_id": a.PatientID,
		"content":    a.Content,
		"updated_at": a.UpdatedAt,
	})
}

type anamnesisBody struct {
	Content string `json:"content"`
}

func (h *ProntuarioHandlers) saveAnamnesis(w http.ResponseWriter, r *http.Request) {
	var b anamnesisBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	pid := r.PathValue("id")
	a, err := h.svc.SaveAnamnesis(r.Context(), pid, b.Content)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "anamnesis", pid, nil)
	httpx.Respond(w, r, http.StatusOK, "Anamnese salva.", map[string]any{
		"patient_id": a.PatientID,
		"updated_at": a.UpdatedAt,
	})
}

func noteView(n *domain.Note) map[string]any {
	return map[string]any{
		"id":         n.ID,
		"patient_id": n.PatientID,
		"session_id": n.SessionID,
		"is_free":    n.IsFree(),
		"content":    n.Content,
		"created_at": n.CreatedAt,
	}
}

func (h *ProntuarioHandlers) listNotes(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	ns, err := h.svc.ListNotes(r.Context(), pid)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionRead, "note", pid, nil)
	views := make([]map[string]any, 0, len(ns))
	for _, n := range ns {
		views = append(views, noteView(n))
	}
	httpx.Respond(w, r, http.StatusOK, "Notas listadas.", views)
}

type noteBody struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
}

func (h *ProntuarioHandlers) addNote(w http.ResponseWriter, r *http.Request) {
	var b noteBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	pid := r.PathValue("id")
	n, err := h.svc.AddNote(r.Context(), pid, b.SessionID, b.Content)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "note", n.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Nota adicionada.", noteView(n))
}

func (h *ProntuarioHandlers) deleteNote(w http.ResponseWriter, r *http.Request) {
	nid := r.PathValue("nid")
	if err := h.svc.DeleteNote(r.Context(), nid); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDelete, "note", nid, nil)
	httpx.Respond(w, r, http.StatusOK, "Nota removida.", nil)
}

func templateView(t *domain.Template) map[string]any {
	return map[string]any{
		"id":         t.ID,
		"name":       t.Name,
		"body_md":    t.BodyMD,
		"created_at": t.CreatedAt,
	}
}

func (h *ProntuarioHandlers) listTemplates(w http.ResponseWriter, r *http.Request) {
	ts, err := h.svc.ListTemplates(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ts))
	for _, t := range ts {
		views = append(views, templateView(t))
	}
	httpx.Respond(w, r, http.StatusOK, "Templates listados.", views)
}

type templateBody struct {
	Name   string `json:"name"`
	BodyMD string `json:"body_md"`
}

func (h *ProntuarioHandlers) createTemplate(w http.ResponseWriter, r *http.Request) {
	var b templateBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	t, err := h.svc.CreateTemplate(r.Context(), b.Name, b.BodyMD)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Template criado.", templateView(t))
}

func (h *ProntuarioHandlers) getTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Template.", templateView(t))
}

// renderTemplate devolve o HTML formatado (versão que se envia).
func (h *ProntuarioHandlers) renderTemplate(w http.ResponseWriter, r *http.Request) {
	html, err := h.svc.RenderTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Template renderizado.", map[string]any{"html": html})
}

type sendTemplateBody struct {
	PatientID string `json:"patient_id"`
}

func (h *ProntuarioHandlers) sendTemplate(w http.ResponseWriter, r *http.Request) {
	var b sendTemplateBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	ts, err := h.svc.SendTemplate(r.Context(), r.PathValue("id"), b.PatientID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Template enviado.", map[string]any{
		"send_id": ts.ID,
		"html":    ts.RenderedHTML,
		"sent_at": ts.SentAt,
	})
}

func (h *ProntuarioHandlers) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteTemplate(r.Context(), r.PathValue("id")); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Template removido.", nil)
}
