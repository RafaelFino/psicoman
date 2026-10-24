package app

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestDayFeedBuild_HojeAgregaEResumo(t *testing.T) {
	ctx := context.Background()
	today := time.Date(2026, 3, 10, 9, 0, 0, 0, clock.Location())

	sessions := newSessionRepo()
	sessions.byID["s1"] = &scheduling.Session{
		ID: "s1", PatientID: "p1", Modality: scheduling.ModalityOnline, Status: scheduling.SessionRealizada,
		StartsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()),
	}
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{
		ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto,
		DueDate: time.Date(2026, 3, 10, 0, 0, 0, 0, clock.Location()), CreatedAt: today,
	}
	patients := newPatientRepo()
	patients.byID["p1"] = &patient.Patient{ID: "p1", Name: "Ana"}

	audit, arepo := newAudit()
	svc := NewDayFeedService(sessions, debts, newPaymentRepo(), newGEDMetaRepo(), &fakeAppointmentRepo{}, patients, audit)
	svc.clock = clock.Fixed{T: today}

	feed, err := svc.Build(ctx, clinical.DayWindowHoje)
	if err != nil {
		t.Fatal(err)
	}
	if feed.Window != clinical.DayWindowHoje {
		t.Fatalf("janela inesperada: %s", feed.Window)
	}
	if feed.Resumo.Total != 1 {
		t.Fatalf("resumo deveria contar 1 sessão, veio %d", feed.Resumo.Total)
	}
	if feed.Resumo.AReceber != 15000 {
		t.Fatalf("a_receber inesperado: %d", feed.Resumo.AReceber)
	}
	if len(arepo.entries) != 1 {
		t.Fatalf("Build deveria auditar a leitura, entries=%d", len(arepo.entries))
	}
}

func TestHistoryPatientHistory_OrdenaEAudita(t *testing.T) {
	ctx := context.Background()

	patients := newPatientRepo()
	patients.byID["p1"] = &patient.Patient{
		ID: "p1", Name: "Ana", ApprovalStatus: patient.PatientApproved,
		CreatedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, clock.Location()),
		UpdatedAt: time.Date(2026, 1, 2, 10, 0, 0, 0, clock.Location()),
	}
	sessions := newSessionRepo()
	sessions.byID["s1"] = &scheduling.Session{
		ID: "s1", PatientID: "p1", Modality: scheduling.ModalityOnline, Status: scheduling.SessionRealizada,
		StartsAt: time.Date(2026, 2, 1, 10, 0, 0, 0, clock.Location()),
	}
	notes := newProntuarioRepo()
	notes.notes["p1"] = []*clinical.Note{{
		ID: "n1", PatientID: "p1", SessionID: "s1", Content: domain.NewSealedText("segredo clínico"),
		CreatedAt: time.Date(2026, 2, 1, 11, 0, 0, 0, clock.Location()),
	}}
	debts := newDebtRepo()
	debts.patients["p1"] = []*billing.Debt{{
		ID: "d1", PatientID: "p1", SessionID: "s1", Amount: 15000, Status: billing.DebtAberto,
		CreatedAt: time.Date(2026, 2, 2, 10, 0, 0, 0, clock.Location()),
	}}

	audit, arepo := newAudit()
	svc := NewHistoryService(patients, sessions, notes, debts, newPaymentRepo(), newGEDMetaRepo(), audit)

	items, err := svc.PatientHistory(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 4 { // cadastro + aprovação + sessão + nota + débito
		t.Fatalf("esperava múltiplos itens, veio %d", len(items))
	}
	// Ordenação desc por data.
	for i := 1; i < len(items); i++ {
		if items[i-1].Data.Before(items[i].Data) {
			t.Fatalf("itens fora de ordem desc em %d", i)
		}
	}
	// Não-vazamento: o resumo da nota nunca contém o claro do SealedText.
	for _, it := range items {
		if it.Resumo == "segredo clínico" || it.Titulo == "segredo clínico" {
			t.Fatal("conteúdo clínico vazou na timeline")
		}
	}
	if acts := arepo.actionsFor("patient_history", "p1"); len(acts) != 1 || acts[0] != domain.AuditActionRead {
		t.Fatalf("esperava auditoria de leitura, veio %v", acts)
	}
}
