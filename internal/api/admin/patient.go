package admin

import (
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PatientHandlers expõe o CRUD de pacientes.
type PatientHandlers struct {
	svc   *service.PatientService
	audit *service.AuditService
}

// NewPatientHandlers cria os handlers de paciente.
func NewPatientHandlers(svc *service.PatientService, audit *service.AuditService) *PatientHandlers {
	return &PatientHandlers{svc: svc, audit: audit}
}

// Register instala as rotas de pacientes no grupo autenticado.
func (h *PatientHandlers) Register(g *api.Group) {
	g.Handle("POST", "/patients", h.create)
	g.Handle("GET", "/patients", h.list)
	g.Handle("GET", "/patients/pending", h.listPending)
	g.Handle("GET", "/patients/{id}", h.get)
	g.Handle("PUT", "/patients/{id}", h.update)
	g.Handle("DELETE", "/patients/{id}", h.delete)
	g.Handle("POST", "/patients/{id}/approve", h.approve)
	g.Handle("POST", "/patients/{id}/reject", h.reject)
}

type patientBody struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	OriginID string `json:"origin_id"`
}

func patientView(p *domain.Patient) map[string]any {
	return map[string]any{
		"id":                p.ID,
		"name":              p.Name,
		"phone":             p.Phone,
		"email":             p.Email,
		"cpf":               p.CPF,
		"origin_id":         p.OriginID,
		"approval_status":   p.ApprovalStatus,
		"can_issue_receipt": p.CanIssueReceipt(),
		"created_at":        p.CreatedAt,
		"updated_at":        p.UpdatedAt,
	}
}

func (h *PatientHandlers) create(w http.ResponseWriter, r *http.Request) {
	var b patientBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	p, err := h.svc.Create(r.Context(), service.CreateInput{
		Name: b.Name, Phone: b.Phone, Email: b.Email, CPF: b.CPF, OriginID: b.OriginID,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionCreate, "patient", p.ID, nil)
	httpx.Respond(w, r, http.StatusCreated, "Paciente cadastrado com sucesso.", patientView(p))
}

func (h *PatientHandlers) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.List(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		views = append(views, patientView(p))
	}
	httpx.Respond(w, r, http.StatusOK, "Pacientes listados.", views)
}

func (h *PatientHandlers) listPending(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.ListPending(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	views := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		views = append(views, patientView(p))
	}
	httpx.Respond(w, r, http.StatusOK, "Pacientes pendentes de aprovação.", views)
}

func (h *PatientHandlers) approve(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Approve(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionApprove, "patient", p.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Paciente aprovado. O acesso já está liberado.", patientView(p))
}

func (h *PatientHandlers) reject(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Reject(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionReject, "patient", p.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Cadastro rejeitado.", patientView(p))
}

func (h *PatientHandlers) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Paciente encontrado.", patientView(p))
}

func (h *PatientHandlers) update(w http.ResponseWriter, r *http.Request) {
	var b patientBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	p, err := h.svc.Update(r.Context(), service.UpdateInput{
		ID: r.PathValue("id"), Name: b.Name, Phone: b.Phone, Email: b.Email, CPF: b.CPF, OriginID: b.OriginID,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionUpdate, "patient", p.ID, nil)
	httpx.Respond(w, r, http.StatusOK, "Paciente atualizado.", patientView(p))
}

func (h *PatientHandlers) delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	_ = h.audit.Record(r.Context(), httpx.Actor(r.Context()), domain.AuditActionDelete, "patient", id, nil)
	httpx.Respond(w, r, http.StatusOK, "Paciente removido.", nil)
}
