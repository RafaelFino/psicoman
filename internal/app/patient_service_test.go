package app

import (
	"context"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func newPatientSvc(repo PatientRepository, c clock.Clock) *PatientService {
	s := NewPatientService(repo)
	s.clock = c
	return s
}

func TestPatientCreate_NormalizaTelefoneEValida(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.Create(ctx, CreateInput{Name: "Ana", Phone: "(11) 99999-0001", Email: "ana@x.com"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Phone != "5511999990001" {
		t.Fatalf("telefone não canônico: %s", p.Phone)
	}
	if p.ApprovalStatus != patient.PatientApproved {
		t.Fatalf("cadastro admin deveria nascer aprovado, veio %s", p.ApprovalStatus)
	}
}

func TestPatientRegisterFromPortal_UpsertNaoDuplica(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	// Paciente já cadastrado pelo terapeuta (aprovado).
	repo.byID["p1"] = &patient.Patient{
		ID: "p1", Name: "Ana", Phone: "5511999990001", Email: "ana@x.com",
		ApprovalStatus: patient.PatientApproved,
	}
	repo.byEmail["ana@x.com"] = repo.byID["p1"]
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.RegisterFromPortal(ctx, PortalRegisterInput{
		Name: "Ana Maria", Phone: "(11) 98888-7777", Email: "ana@x.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.creates != 0 {
		t.Fatalf("upsert por email não deveria criar novo paciente, creates=%d", repo.creates)
	}
	if p.ID != "p1" {
		t.Fatalf("deveria vincular ao registro existente, veio %s", p.ID)
	}
	if p.ApprovalStatus != patient.PatientApproved {
		t.Fatalf("vínculo não deveria rebaixar estado aprovado, veio %s", p.ApprovalStatus)
	}
}

func TestPatientRegisterFromPortal_NovoNascePendente(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.RegisterFromPortal(ctx, PortalRegisterInput{
		Name: "Bento", Phone: "11988887777", Email: "bento@x.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ApprovalStatus != patient.PatientPending {
		t.Fatalf("auto-cadastro deveria nascer pendente, veio %s", p.ApprovalStatus)
	}
	if repo.creates != 1 {
		t.Fatalf("esperava 1 criação, creates=%d", repo.creates)
	}
}

func TestPatientApprove_PendenteViraAprovado(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	repo.byID["p1"] = &patient.Patient{ID: "p1", Email: "ana@x.com", ApprovalStatus: patient.PatientPending}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.Approve(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ApprovalStatus != patient.PatientApproved {
		t.Fatalf("esperava aprovado, veio %s", p.ApprovalStatus)
	}
}

func TestPatientApprove_Idempotente(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	repo.byID["p1"] = &patient.Patient{ID: "p1", Email: "ana@x.com", ApprovalStatus: patient.PatientApproved}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.Approve(ctx, "p1")
	if err != nil {
		t.Fatalf("aprovar já-aprovado deve ser idempotente, veio: %v", err)
	}
	if p.ApprovalStatus != patient.PatientApproved {
		t.Fatalf("esperava aprovado, veio %s", p.ApprovalStatus)
	}
}

func TestPatientReject_PendenteViraRejeitado(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	repo.byID["p1"] = &patient.Patient{ID: "p1", Email: "ana@x.com", ApprovalStatus: patient.PatientPending}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	p, err := svc.Reject(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ApprovalStatus != patient.PatientRejected {
		t.Fatalf("esperava rejeitado, veio %s", p.ApprovalStatus)
	}
}

func TestPatientReject_RejeitadoNaoViraAprovado(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	repo.byID["p1"] = &patient.Patient{ID: "p1", Email: "ana@x.com", ApprovalStatus: patient.PatientRejected}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	// Estado final não retrocede: rejeitado→aprovado é transição barrada.
	if _, err := svc.Approve(ctx, "p1"); err == nil {
		t.Fatal("esperava transição barrada de rejeitado→aprovado")
	}
}

func TestPatientCreate_EmailDuplicadoConflito(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	repo.byEmail["ana@x.com"] = &patient.Patient{ID: "p1", Email: "ana@x.com"}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))

	_, err := svc.Create(ctx, CreateInput{Name: "Ana", Phone: "11999990001", Email: "ana@x.com"})
	if err == nil {
		t.Fatal("esperava conflito de email")
	}
}
