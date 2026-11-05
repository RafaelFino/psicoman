package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// trackingApptRepo guarda os pedidos em memória.
type trackingApptRepo struct {
	byID map[string]*scheduling.AppointmentRequest
}

func newTrackingApptRepo() *trackingApptRepo {
	return &trackingApptRepo{byID: map[string]*scheduling.AppointmentRequest{}}
}

func (f *trackingApptRepo) Create(_ context.Context, a *scheduling.AppointmentRequest) error {
	cp := *a
	f.byID[a.ID] = &cp
	return nil
}
func (f *trackingApptRepo) Get(_ context.Context, id string) (*scheduling.AppointmentRequest, error) {
	a, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	return &cp, nil
}
func (f *trackingApptRepo) ListPending(context.Context) ([]*scheduling.AppointmentRequest, error) {
	var out []*scheduling.AppointmentRequest
	for _, a := range f.byID {
		if a.Status == scheduling.ApptPendente {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *trackingApptRepo) ListByPatient(_ context.Context, pid string) ([]*scheduling.AppointmentRequest, error) {
	var out []*scheduling.AppointmentRequest
	for _, a := range f.byID {
		if a.PatientID == pid {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *trackingApptRepo) UpdateStatus(_ context.Context, id, status string) error {
	f.byID[id].Status = status
	return nil
}

type apptHarness struct {
	svc      *AppointmentService
	repo     *trackingApptRepo
	sessions *fakeSessionRepo
	conflict *fakeConflictChecker
	locs     *trackingLocationRepo
}

func newApptHarness(t *testing.T) *apptHarness {
	t.Helper()
	patients := newPatientRepo()
	patients.byID["ok"] = &patient.Patient{ID: "ok", Name: "A", Email: "ok@x.com", ApprovalStatus: patient.PatientApproved}
	patients.byID["pend"] = &patient.Patient{ID: "pend", Name: "B", Email: "pend@x.com", ApprovalStatus: patient.PatientPending}
	sessRepo := newSessionRepo()
	sessSvc := newSessionSvc(sessRepo, patients, fixedAt(2026, 6, 1))
	conflict := &fakeConflictChecker{}
	sessSvc.SetConflictChecker(conflict)
	locs := newTrackingLocationRepo()
	locs.byID["loc1"] = &scheduling.Location{ID: "loc1", Name: "C", Modality: scheduling.ModalityPresencial}
	repo := newTrackingApptRepo()
	return &apptHarness{
		svc:  NewAppointmentService(repo, patients, locs, sessSvc),
		repo: repo, sessions: sessRepo, conflict: conflict, locs: locs,
	}
}

func slot() (time.Time, time.Time) {
	s := time.Date(2026, 7, 1, 14, 0, 0, 0, clock.Location())
	return s, s.Add(time.Hour)
}

func TestApptRequest_PacienteAprovadoCriaPendente(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	a, err := h.svc.Request(ctx, "ok", AppointmentInput{LocationID: "loc1", SlotStart: s, SlotEnd: e, Note: "manhã"})
	if err != nil || a.Status != scheduling.ApptPendente || a.ID == "" {
		t.Fatalf("Request: %v / %+v", err, a)
	}
}

func TestApptRequest_Rejeicoes(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	cases := []struct {
		name string
		pid  string
		in   AppointmentInput
		want error
	}{
		{"paciente pendente", "pend", AppointmentInput{SlotStart: s, SlotEnd: e}, ErrValidation},
		{"paciente inexistente", "xx", AppointmentInput{SlotStart: s, SlotEnd: e}, ErrNotFound},
		{"local inexistente", "ok", AppointmentInput{LocationID: "nao", SlotStart: s, SlotEnd: e}, ErrValidation},
		{"slot invertido", "ok", AppointmentInput{SlotStart: e, SlotEnd: s}, ErrValidation},
		{"slot vazio", "ok", AppointmentInput{}, ErrValidation},
	}
	for _, c := range cases {
		if _, err := h.svc.Request(ctx, c.pid, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: quero %v, veio %v", c.name, c.want, err)
		}
	}
	if len(h.repo.byID) != 0 {
		t.Errorf("nenhum pedido deveria ter sido gravado, há %d", len(h.repo.byID))
	}
}

func TestApptConfirm_CriaSessaoAgendadaEMarcaConfirmado(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	a, _ := h.svc.Request(ctx, "ok", AppointmentInput{LocationID: "loc1", SlotStart: s, SlotEnd: e})

	sess, err := h.svc.Confirm(ctx, a.ID)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if sess.Status != scheduling.SessionAgendada || sess.RequestID != a.ID || sess.Modality != scheduling.ModalityPresencial {
		t.Errorf("sessão inesperada: %+v", sess)
	}
	if h.repo.byID[a.ID].Status != scheduling.ApptConfirmado {
		t.Errorf("pedido deveria estar confirmado: %s", h.repo.byID[a.ID].Status)
	}
	// Segunda confirmação → conflito (já respondido), sem nova sessão.
	if _, err := h.svc.Confirm(ctx, a.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("reconfirmar deveria ser ErrConflict, veio %v", err)
	}
	if h.sessions.creates != 1 {
		t.Errorf("deveria haver 1 sessão criada, há %d", h.sessions.creates)
	}
}

func TestApptConfirm_ConflitoDeAgendaMantemPendente(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	a, _ := h.svc.Request(ctx, "ok", AppointmentInput{SlotStart: s, SlotEnd: e})
	h.conflict.conflict = true

	if _, err := h.svc.Confirm(ctx, a.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflito de agenda deveria ser ErrConflict, veio %v", err)
	}
	if h.repo.byID[a.ID].Status != scheduling.ApptPendente {
		t.Errorf("pedido deveria seguir pendente, está %s", h.repo.byID[a.ID].Status)
	}
	if h.sessions.creates != 0 {
		t.Errorf("nenhuma sessão deveria ser criada, há %d", h.sessions.creates)
	}
}

func TestApptRefuse(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	a, _ := h.svc.Request(ctx, "ok", AppointmentInput{SlotStart: s, SlotEnd: e})

	if err := h.svc.Refuse(ctx, a.ID); err != nil {
		t.Fatalf("Refuse: %v", err)
	}
	if h.repo.byID[a.ID].Status != scheduling.ApptRecusado {
		t.Errorf("pedido deveria estar recusado")
	}
	if err := h.svc.Refuse(ctx, a.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("recusar de novo deveria ser ErrConflict, veio %v", err)
	}
	if _, err := h.svc.Confirm(ctx, a.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("confirmar recusado deveria ser ErrConflict, veio %v", err)
	}
	if err := h.svc.Refuse(ctx, "nao"); !errors.Is(err, ErrNotFound) {
		t.Errorf("inexistente deveria ser ErrNotFound, veio %v", err)
	}
}

func TestApptListagens(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	s, e := slot()
	a1, _ := h.svc.Request(ctx, "ok", AppointmentInput{SlotStart: s, SlotEnd: e})
	h.svc.Request(ctx, "ok", AppointmentInput{SlotStart: s.Add(24 * time.Hour), SlotEnd: e.Add(24 * time.Hour)})
	_ = h.svc.Refuse(ctx, a1.ID)

	if p, _ := h.svc.ListPending(ctx); len(p) != 1 {
		t.Errorf("pendentes: quero 1, veio %d", len(p))
	}
	if all, _ := h.svc.ListByPatient(ctx, "ok"); len(all) != 2 {
		t.Errorf("do paciente: quero 2, veio %d", len(all))
	}
}
