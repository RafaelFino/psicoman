package app

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func fixedAt(y int, m time.Month, d int) clock.Fixed {
	return clock.Fixed{T: time.Date(y, m, d, 10, 0, 0, 0, clock.Location())}
}

func newBilling(debts DebtRepository, plans PlanRepository, c clock.Clock) (*BillingService, *fakeAuditRepo) {
	audit, arepo := newAudit()
	audit.clock = c
	s := NewBillingService(debts, plans, audit)
	s.clock = c
	return s, arepo
}

func finishedSession(patientID, id string, bill bool) *scheduling.Session {
	return &scheduling.Session{
		ID: id, PatientID: patientID, Status: scheduling.SessionRealizada, Bill: bill,
		StartsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()),
	}
}

func TestBillingOnSessionFinished_PerSessionIdempotente(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: &billing.Plan{ID: "plan1", PatientID: "p1", Type: billing.PlanPorConsulta, Amount: 15000}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	sess := finishedSession("p1", "s1", true)
	if err := svc.OnSessionFinished(ctx, sess); err != nil {
		t.Fatalf("1ª chamada: %v", err)
	}
	if debts.inserts != 1 {
		t.Fatalf("esperava 1 débito inserido, veio %d", debts.inserts)
	}
	// 2ª chamada (reprocesso) não duplica: mesma idempotency_key por session_id.
	if err := svc.OnSessionFinished(ctx, sess); err != nil {
		t.Fatalf("2ª chamada: %v", err)
	}
	if debts.inserts != 1 {
		t.Fatalf("reprocesso duplicou débito: inserts=%d", debts.inserts)
	}
}

func TestBillingOnSessionFinished_BillFalseNaoGera(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: &billing.Plan{ID: "plan1", PatientID: "p1", Type: billing.PlanPorConsulta, Amount: 15000}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, finishedSession("p1", "s1", false)); err != nil {
		t.Fatal(err)
	}
	if debts.inserts != 0 {
		t.Fatalf("opt-out deveria suprimir débito, inserts=%d", debts.inserts)
	}
}

func TestBillingOnSessionFinished_SocialNaoGera(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: &billing.Plan{ID: "plan1", PatientID: "p1", Type: billing.PlanAtendimentoSoc}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, finishedSession("p1", "s1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.inserts != 0 {
		t.Fatalf("social não deveria gerar débito, inserts=%d", debts.inserts)
	}
}

func TestBillingOnSessionFinished_FechadoNaoGeraPorSessao(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: &billing.Plan{ID: "plan1", PatientID: "p1", Type: billing.PlanFechadoMensal, Amount: 50000}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, finishedSession("p1", "s1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.inserts != 0 {
		t.Fatalf("plano fechado não gera por sessão, inserts=%d", debts.inserts)
	}
}

func TestBillingOnSessionFinished_SemPlanoAuditaSkip(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: nil} // GetActiveByPatient ⇒ ErrNotFound
	svc, arepo := newBilling(debts, plans, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, finishedSession("p1", "s1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.inserts != 0 {
		t.Fatalf("sem plano não gera débito, inserts=%d", debts.inserts)
	}
	acts := arepo.actionsFor("session", "s1")
	if len(acts) != 1 {
		t.Fatalf("esperava auditar o skip sem plano, veio %v", acts)
	}
}

func TestBillingCloseCycles_GeraUmaVezPorPeriodo(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	// due_day 1; ref dia 10/03 já alcançou o vencimento do mês.
	plan := &billing.Plan{ID: "planM", PatientID: "p1", Type: billing.PlanFechadoMensal, Amount: 50000, DueDay: 1}
	plans := &fakePlanRepo{fixedCycle: []*billing.Plan{plan}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	ref := time.Date(2026, 3, 10, 0, 0, 0, 0, clock.Location())
	n, err := svc.CloseCycles(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || debts.inserts != 1 {
		t.Fatalf("esperava 1 débito de ciclo, n=%d inserts=%d", n, debts.inserts)
	}
	// Reprocessar o mesmo período não duplica (idempotência por plan+period).
	n2, err := svc.CloseCycles(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 || debts.inserts != 1 {
		t.Fatalf("reprocesso duplicou ciclo, n2=%d inserts=%d", n2, debts.inserts)
	}
}

func TestBillingCloseCycles_DueNaoAlcancado(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	// due_day 28; ref dia 10/03 ainda não alcançou o vencimento.
	plan := &billing.Plan{ID: "planM", PatientID: "p1", Type: billing.PlanFechadoMensal, Amount: 50000, DueDay: 28}
	plans := &fakePlanRepo{fixedCycle: []*billing.Plan{plan}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	ref := time.Date(2026, 3, 10, 0, 0, 0, 0, clock.Location())
	n, err := svc.CloseCycles(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || debts.inserts != 0 {
		t.Fatalf("due não alcançado não deveria gerar, n=%d inserts=%d", n, debts.inserts)
	}
}

func TestBillingCloseCycles_PeriodoVazio(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{fixedCycle: nil}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))

	n, err := svc.CloseCycles(ctx, time.Date(2026, 3, 10, 0, 0, 0, 0, clock.Location()))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("sem planos fechados deveria gerar 0, veio %d", n)
	}
}
