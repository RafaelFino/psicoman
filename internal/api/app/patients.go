package app

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
)

// Health responde no envelope (mantém 200 para o smoke de boot).
func Health(e *core.RequestEvent) error {
	return httpx.RespondOK(e, "Psicoman no ar.", map[string]any{"status": "ok"})
}

type patientCreateBody struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	OriginID string `json:"origin_id"`
}

type patientUpdateBody struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	OriginID string `json:"origin_id"`
}

type noteBody struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id"`
}

type anamnesisBody struct {
	Content string `json:"content"`
}

// ListPatients GET /v1/app/patients.
func (h *Handlers) ListPatients(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	ps, err := h.svc.Patients.List(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Pacientes listados.", toPatientDTOs(ps))
}

// CreatePatient POST /v1/app/patients.
func (h *Handlers) CreatePatient(e *core.RequestEvent) error {
	var body patientCreateBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.Create(ctx, app.CreateInput{
		Name:     body.Name,
		Phone:    body.Phone,
		Email:    body.Email,
		CPF:      body.CPF,
		OriginID: body.OriginID,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Paciente cadastrado.", toPatientDTO(p))
}

// GetPatientChart GET /v1/app/patients/{id} — paciente + prontuário em claro.
func (h *Handlers) GetPatientChart(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.Get(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	anam, notes, err := h.svc.Prontuario.GetChart(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	chart := PatientChartDTO{
		Patient: toPatientDTO(p),
		Notes:   toNoteDTOs(notes),
	}
	if anam != nil {
		dto := toAnamnesisDTO(anam)
		chart.Anamnesis = &dto
	}
	return httpx.RespondOK(e, "Prontuário do paciente.", chart)
}

// UpdatePatient PATCH /v1/app/patients/{id} — full-replace dos campos editáveis.
func (h *Handlers) UpdatePatient(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body patientUpdateBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.Update(ctx, app.UpdateInput{
		ID:       id, // o id do path é a fonte de verdade
		Name:     body.Name,
		Phone:    body.Phone,
		Email:    body.Email,
		CPF:      body.CPF,
		OriginID: body.OriginID,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Paciente atualizado.", toPatientDTO(p))
}

// DeletePatient DELETE /v1/app/patients/{id}.
func (h *Handlers) DeletePatient(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	if err := h.svc.Patients.Delete(ctx, id); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Paciente removido.", nil)
}

// ListPendingPatients GET /v1/app/patients/pending — fila de aprovação.
func (h *Handlers) ListPendingPatients(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	ps, err := h.svc.Patients.ListPending(ctx)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Pacientes pendentes de aprovação.", toPatientDTOs(ps))
}

// ApprovePatient POST /v1/app/patients/{id}/approve.
func (h *Handlers) ApprovePatient(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.Approve(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Cadastro aprovado.", toPatientDTO(p))
}

// RejectPatient POST /v1/app/patients/{id}/reject.
func (h *Handlers) RejectPatient(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.Reject(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Cadastro rejeitado.", toPatientDTO(p))
}

// AddNote POST /v1/app/patients/{id}/notes.
func (h *Handlers) AddNote(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body noteBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	n, err := h.svc.Prontuario.AddNote(ctx, app.AddNoteInput{
		PatientID: id,
		SessionID: body.SessionID,
		Content:   body.Content,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Nota adicionada.", toNoteDTO(n))
}

// UpsertAnamnesis PUT /v1/app/patients/{id}/anamnesis.
func (h *Handlers) UpsertAnamnesis(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	var body anamnesisBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	a, err := h.svc.Prontuario.UpsertAnamnesis(ctx, app.UpsertAnamnesisInput{
		PatientID: id,
		Content:   body.Content,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Anamnese registrada.", toAnamnesisDTO(a))
}

// ListPatientDebts GET /v1/app/patients/{id}/debts.
func (h *Handlers) ListPatientDebts(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	ds, err := h.svc.Billing.ListDebtsByPatient(ctx, id)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Débitos do paciente.", toDebtDTOs(ds))
}
