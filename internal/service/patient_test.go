package service

import (
	"context"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// fakePatientRepo é um PatientRepository em memória para os testes de serviço.
type fakePatientRepo struct {
	byID    map[string]*domain.Patient
	byEmail map[string]*domain.Patient
}

func newFakePatientRepo() *fakePatientRepo {
	return &fakePatientRepo{byID: map[string]*domain.Patient{}, byEmail: map[string]*domain.Patient{}}
}

func (f *fakePatientRepo) put(p *domain.Patient) {
	cp := *p
	f.byID[p.ID] = &cp
	f.byEmail[p.Email] = &cp
}

func (f *fakePatientRepo) Create(_ context.Context, p *domain.Patient) error {
	f.put(p)
	return nil
}

func (f *fakePatientRepo) Update(_ context.Context, p *domain.Patient) error {
	if _, ok := f.byID[p.ID]; !ok {
		return ErrNotFound
	}
	f.put(p)
	return nil
}

func (f *fakePatientRepo) Get(_ context.Context, id string) (*domain.Patient, error) {
	if p, ok := f.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (f *fakePatientRepo) GetByEmail(_ context.Context, email string) (*domain.Patient, error) {
	if p, ok := f.byEmail[email]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (f *fakePatientRepo) List(_ context.Context) ([]*domain.Patient, error) {
	out := make([]*domain.Patient, 0, len(f.byID))
	for _, p := range f.byID {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakePatientRepo) ListByApproval(_ context.Context, status string) ([]*domain.Patient, error) {
	var out []*domain.Patient
	for _, p := range f.byID {
		if p.ApprovalStatus == status {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakePatientRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

func (f *fakePatientRepo) EmailExists(_ context.Context, email, exceptID string) (bool, error) {
	if p, ok := f.byEmail[email]; ok && p.ID != exceptID {
		return true, nil
	}
	return false, nil
}

func (f *fakePatientRepo) CPFExists(_ context.Context, cpf, exceptID string) (bool, error) {
	for _, p := range f.byID {
		if p.CPF == cpf && p.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}

func newPatientSvc() *PatientService { return NewPatientService(newFakePatientRepo()) }

func TestCreateAdminStartsApproved(t *testing.T) {
	svc := newPatientSvc()
	p, err := svc.Create(context.Background(), CreateInput{Name: "Ana", Phone: "1199", Email: "ana@x.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ApprovalStatus != domain.PatientApproved {
		t.Errorf("cadastro admin deveria nascer aprovado, veio %q", p.ApprovalStatus)
	}
}

func TestRegisterFromPortalStartsPending(t *testing.T) {
	svc := newPatientSvc()
	p, err := svc.RegisterFromPortal(context.Background(), PortalRegisterInput{Name: "Bia", Phone: "1188", Email: "bia@x.com"})
	if err != nil {
		t.Fatalf("RegisterFromPortal: %v", err)
	}
	if p.ApprovalStatus != domain.PatientPending {
		t.Errorf("auto-cadastro deveria nascer pendente, veio %q", p.ApprovalStatus)
	}
}

func TestRegisterFromPortalDoesNotDowngradeApproved(t *testing.T) {
	svc := newPatientSvc()
	// Terapeuta cadastra (aprovado).
	created, err := svc.Create(context.Background(), CreateInput{Name: "Caio", Phone: "1177", Email: "caio@x.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Paciente se auto-cadastra pelo portal com o mesmo email.
	linked, err := svc.RegisterFromPortal(context.Background(), PortalRegisterInput{Name: "Caio", Phone: "1177", Email: "caio@x.com"})
	if err != nil {
		t.Fatalf("RegisterFromPortal (vínculo): %v", err)
	}
	if linked.ID != created.ID {
		t.Errorf("deveria vincular ao registro existente, não duplicar")
	}
	if linked.ApprovalStatus != domain.PatientApproved {
		t.Errorf("vínculo por email não deveria rebaixar aprovado, veio %q", linked.ApprovalStatus)
	}
}

func TestApproveAndReject(t *testing.T) {
	ctx := context.Background()
	t.Run("aprova pendente", func(t *testing.T) {
		svc := newPatientSvc()
		p, _ := svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "D", Phone: "1", Email: "d@x.com"})
		got, err := svc.Approve(ctx, p.ID)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if got.ApprovalStatus != domain.PatientApproved {
			t.Errorf("esperava aprovado, veio %q", got.ApprovalStatus)
		}
	})
	t.Run("rejeita pendente", func(t *testing.T) {
		svc := newPatientSvc()
		p, _ := svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "E", Phone: "1", Email: "e@x.com"})
		got, err := svc.Reject(ctx, p.ID)
		if err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if got.ApprovalStatus != domain.PatientRejected {
			t.Errorf("esperava rejeitado, veio %q", got.ApprovalStatus)
		}
	})
	t.Run("rejeitar aprovado falha", func(t *testing.T) {
		svc := newPatientSvc()
		p, _ := svc.Create(ctx, CreateInput{Name: "F", Phone: "1", Email: "f@x.com"})
		if _, err := svc.Reject(ctx, p.ID); err == nil {
			t.Error("não deveria poder rejeitar um cadastro aprovado")
		}
	})
	t.Run("aprovar já aprovado é idempotente", func(t *testing.T) {
		svc := newPatientSvc()
		p, _ := svc.Create(ctx, CreateInput{Name: "G", Phone: "1", Email: "g@x.com"})
		if _, err := svc.Approve(ctx, p.ID); err != nil {
			t.Errorf("aprovar já aprovado deveria ser no-op, veio %v", err)
		}
	})
}

func TestListPending(t *testing.T) {
	ctx := context.Background()
	svc := newPatientSvc()
	_, _ = svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "P1", Phone: "1", Email: "p1@x.com"})
	_, _ = svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "P2", Phone: "1", Email: "p2@x.com"})
	_, _ = svc.Create(ctx, CreateInput{Name: "A1", Phone: "1", Email: "a1@x.com"})
	pend, err := svc.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pend) != 2 {
		t.Errorf("esperava 2 pendentes, veio %d", len(pend))
	}
}
