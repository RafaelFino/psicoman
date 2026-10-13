package service

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// fakeDebtRepo é um DebtRepository em memória com idempotência por chave.
type fakeDebtRepo struct {
	byKey map[string]*domain.Debt
}

func newFakeDebtRepo() *fakeDebtRepo { return &fakeDebtRepo{byKey: map[string]*domain.Debt{}} }

func (f *fakeDebtRepo) InsertIfAbsent(_ context.Context, d *domain.Debt) (bool, error) {
	if _, ok := f.byKey[d.IdempotencyKey]; ok {
		return false, nil
	}
	f.byKey[d.IdempotencyKey] = d
	return true, nil
}
func (f *fakeDebtRepo) Get(context.Context, string) (*domain.Debt, error) { return nil, ErrNotFound }
func (f *fakeDebtRepo) GetByIdempotencyKey(_ context.Context, k string) (*domain.Debt, error) {
	if d, ok := f.byKey[k]; ok {
		return d, nil
	}
	return nil, ErrNotFound
}
func (f *fakeDebtRepo) List(context.Context) ([]*domain.Debt, error) { return nil, nil }
func (f *fakeDebtRepo) ListByPatient(context.Context, string) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *fakeDebtRepo) ListBySessionIDs(context.Context, []string) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *fakeDebtRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *fakeDebtRepo) ListDueBetween(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *fakeDebtRepo) UpdateStatus(context.Context, string, string) error { return nil }
func (f *fakeDebtRepo) SetPDF(context.Context, string, string) error       { return nil }
func (f *fakeDebtRepo) count() int                                         { return len(f.byKey) }

// fakePlanRepo devolve um plano fixo configurado.
type fakePlanRepo struct {
	active     *domain.Plan
	fixedCycle []*domain.Plan
}

func (f *fakePlanRepo) Create(context.Context, *domain.Plan) error { return nil }
func (f *fakePlanRepo) Update(context.Context, *domain.Plan) error { return nil }
func (f *fakePlanRepo) Get(context.Context, string) (*domain.Plan, error) {
	return nil, ErrNotFound
}
func (f *fakePlanRepo) GetActiveByPatient(context.Context, string, time.Time) (*domain.Plan, error) {
	if f.active == nil {
		return nil, ErrNotFound
	}
	return f.active, nil
}
func (f *fakePlanRepo) ListByPatient(context.Context, string) ([]*domain.Plan, error) {
	return nil, nil
}
func (f *fakePlanRepo) ListFixedCycleActive(context.Context, time.Time) ([]*domain.Plan, error) {
	return f.fixedCycle, nil
}
func (f *fakePlanRepo) SoftDelete(context.Context, string) error { return nil }

// fakeAuditRepo descarta tudo.
type fakeAuditRepo struct{}

func (fakeAuditRepo) Insert(context.Context, *domain.AuditLog) error { return nil }
func (fakeAuditRepo) List(context.Context, int) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (fakeAuditRepo) ListByEntity(context.Context, string, string) ([]*domain.AuditLog, error) {
	return nil, nil
}

// capturingAuditRepo guarda as entradas para inspeção.
type capturingAuditRepo struct{ entries []*domain.AuditLog }

func (c *capturingAuditRepo) Insert(_ context.Context, e *domain.AuditLog) error {
	c.entries = append(c.entries, e)
	return nil
}
func (c *capturingAuditRepo) List(context.Context, int) ([]*domain.AuditLog, error) {
	return c.entries, nil
}
func (c *capturingAuditRepo) ListByEntity(_ context.Context, entity, entityID string) ([]*domain.AuditLog, error) {
	var out []*domain.AuditLog
	for _, e := range c.entries {
		if e.Entity == entity && e.EntityID == entityID {
			out = append(out, e)
		}
	}
	return out, nil
}

func newSession(patientID string, bill bool) *domain.Session {
	now := time.Now()
	return &domain.Session{
		ID: ulid.New(), PatientID: patientID, Status: domain.SessionRealizada,
		Bill: bill, StartsAt: now, EndsAt: now.Add(time.Hour),
	}
}

func TestBillingPerSessionIdempotent(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: &domain.Plan{
		ID: "plan1", Type: domain.PlanPorConsulta, Amount: 15000, StartsAt: time.Now().Add(-time.Hour),
	}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	sess := newSession("p1", true)
	// Primeira finalização gera 1 débito.
	if err := svc.OnSessionFinished(context.Background(), sess); err != nil {
		t.Fatalf("OnSessionFinished: %v", err)
	}
	// Segunda (reprocessamento) não duplica.
	if err := svc.OnSessionFinished(context.Background(), sess); err != nil {
		t.Fatalf("OnSessionFinished 2: %v", err)
	}
	if debts.count() != 1 {
		t.Errorf("débitos = %d, quer 1 (idempotente)", debts.count())
	}
}

func TestBillingNoBillFlag(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: &domain.Plan{ID: "p", Type: domain.PlanPorConsulta, Amount: 100, StartsAt: time.Now().Add(-time.Hour)}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	// bill=false → não gera débito.
	if err := svc.OnSessionFinished(context.Background(), newSession("p1", false)); err != nil {
		t.Fatal(err)
	}
	if debts.count() != 0 {
		t.Errorf("débitos = %d, quer 0 (sem flag bill)", debts.count())
	}
}

func TestBillingSocialNeverGenerates(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: &domain.Plan{ID: "p", Type: domain.PlanAtendimentoSoc, StartsAt: time.Now().Add(-time.Hour)}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	// Mesmo com bill=true, social nunca gera.
	if err := svc.OnSessionFinished(context.Background(), newSession("p1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.count() != 0 {
		t.Errorf("débitos = %d, quer 0 (atendimento social)", debts.count())
	}
}

func TestBillingFixedCycleNotPerSession(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: &domain.Plan{ID: "p", Type: domain.PlanFechadoMensal, Amount: 50000, StartsAt: time.Now().Add(-time.Hour)}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	// Plano fechado NÃO gera na finalização de sessão.
	if err := svc.OnSessionFinished(context.Background(), newSession("p1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.count() != 0 {
		t.Errorf("débitos = %d, quer 0 (fechado cobra por ciclo)", debts.count())
	}
}

func TestBillingCloseCyclesIdempotent(t *testing.T) {
	debts := newFakeDebtRepo()
	fixed := &domain.Plan{ID: "planM", PatientID: "p1", Type: domain.PlanFechadoMensal, Amount: 50000, StartsAt: time.Now().Add(-time.Hour)}
	plans := &fakePlanRepo{fixedCycle: []*domain.Plan{fixed}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	ref := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	n1, err := svc.CloseCycles(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 {
		t.Errorf("primeiro fechamento criou %d, quer 1", n1)
	}
	// Reexecutar no mesmo período não duplica.
	n2, err := svc.CloseCycles(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("segundo fechamento criou %d, quer 0 (idempotente)", n2)
	}
	if debts.count() != 1 {
		t.Errorf("débitos totais = %d, quer 1", debts.count())
	}

	// Período diferente gera novo débito.
	ref2 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	n3, _ := svc.CloseCycles(context.Background(), ref2)
	if n3 != 1 {
		t.Errorf("fechamento de novo mês criou %d, quer 1", n3)
	}
}

func TestBillingNoPlanSkipObservable(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: nil} // sem plano vigente
	audit := &capturingAuditRepo{}
	svc := NewBillingService(debts, plans, NewAuditService(audit))

	if err := svc.OnSessionFinished(context.Background(), newSession("p1", true)); err != nil {
		t.Fatal(err)
	}
	if debts.count() != 0 {
		t.Errorf("débitos = %d, quer 0 (sem plano)", debts.count())
	}
	// Deve registrar o skip observável (sem conteúdo clínico).
	found := false
	for _, e := range audit.entries {
		if e.Metadata != nil {
			if v, ok := e.Metadata["debt_skip_no_plan"]; ok && v == true {
				found = true
			}
		}
	}
	if !found {
		t.Error("esperava audit com debt_skip_no_plan=true")
	}
}

func TestBillingOptOutExplicit(t *testing.T) {
	debts := newFakeDebtRepo()
	plans := &fakePlanRepo{active: &domain.Plan{ID: "p", Type: domain.PlanPorConsulta, Amount: 100, StartsAt: time.Now().Add(-time.Hour)}}
	svc := NewBillingService(debts, plans, NewAuditService(fakeAuditRepo{}))

	// Opt-out explícito (bill=false): não gera, mesmo com plano por-sessão.
	if err := svc.OnSessionFinished(context.Background(), newSession("p1", false)); err != nil {
		t.Fatal(err)
	}
	if debts.count() != 0 {
		t.Errorf("débitos = %d, quer 0 (opt-out)", debts.count())
	}
}

func TestCloseCyclesDueDayTrigger(t *testing.T) {
	// Plano mensal com vencimento no dia 10.
	fixed := &domain.Plan{ID: "planM", PatientID: "p1", Type: domain.PlanFechadoMensal, Amount: 50000, DueDay: 10, StartsAt: time.Now().Add(-time.Hour)}
	loc := clock.Location()

	// Antes do vencimento (dia 9): não gera.
	t.Run("antes do vencimento", func(t *testing.T) {
		debts := newFakeDebtRepo()
		svc := NewBillingService(debts, &fakePlanRepo{fixedCycle: []*domain.Plan{fixed}}, NewAuditService(fakeAuditRepo{}))
		ref := time.Date(2026, 3, 9, 10, 0, 0, 0, loc)
		n, err := svc.CloseCycles(context.Background(), ref)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 || debts.count() != 0 {
			t.Errorf("antes do vencimento gerou %d (count %d), quer 0", n, debts.count())
		}
	})

	// No dia e depois: gera 1 e não duplica.
	t.Run("a partir do vencimento idempotente", func(t *testing.T) {
		debts := newFakeDebtRepo()
		svc := NewBillingService(debts, &fakePlanRepo{fixedCycle: []*domain.Plan{fixed}}, NewAuditService(fakeAuditRepo{}))
		ref := time.Date(2026, 3, 10, 10, 0, 0, 0, loc)
		n, err := svc.CloseCycles(context.Background(), ref)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("no vencimento gerou %d, quer 1", n)
		}
		// Rodar vários dias depois não duplica.
		for _, d := range []int{11, 12, 20, 28} {
			ref2 := time.Date(2026, 3, d, 10, 0, 0, 0, loc)
			n2, _ := svc.CloseCycles(context.Background(), ref2)
			if n2 != 0 {
				t.Errorf("dia %d gerou %d, quer 0 (idempotente)", d, n2)
			}
		}
		if debts.count() != 1 {
			t.Errorf("débitos totais = %d, quer 1", debts.count())
		}
		// due_date = vencimento real do período (2026-03-10).
		d, err := debts.GetByIdempotencyKey(context.Background(), domain.CycleIdempotencyKey(fixed.ID, "2026-03"))
		if err != nil {
			t.Fatalf("buscando débito: %v", err)
		}
		if d.DueDate.Year() != 2026 || d.DueDate.Month() != time.March || d.DueDate.Day() != 10 {
			t.Errorf("due_date = %v, quer 2026-03-10", d.DueDate)
		}
	})
}

func TestCloseCyclesQuarterlyTriggerFirstMonth(t *testing.T) {
	// Trimestral, vencimento dia 15. Q1 começa em janeiro.
	fixed := &domain.Plan{ID: "planT", PatientID: "p1", Type: domain.PlanFechadoTrim, Amount: 150000, DueDay: 15, StartsAt: time.Date(2026, 1, 1, 0, 0, 0, 0, clock.Location())}
	loc := clock.Location()
	debts := newFakeDebtRepo()
	svc := NewBillingService(debts, &fakePlanRepo{fixedCycle: []*domain.Plan{fixed}}, NewAuditService(fakeAuditRepo{}))

	// Em fevereiro (depois do dia 15 de janeiro): já venceu o trimestre Q1.
	ref := time.Date(2026, 2, 1, 10, 0, 0, 0, loc)
	n, err := svc.CloseCycles(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("trimestral gerou %d, quer 1", n)
	}
	// due_date ancorada no 1º mês do trimestre (janeiro, dia 15).
	d, _ := debts.GetByIdempotencyKey(context.Background(), domain.CycleIdempotencyKey(fixed.ID, "2026-Q1"))
	if d == nil || d.DueDate.Month() != time.January || d.DueDate.Day() != 15 {
		t.Errorf("due_date trimestral = %v, quer 2026-01-15", d)
	}

	// Antes do vencimento do trimestre: 5 de janeiro, não gera.
	debts2 := newFakeDebtRepo()
	svc2 := NewBillingService(debts2, &fakePlanRepo{fixedCycle: []*domain.Plan{fixed}}, NewAuditService(fakeAuditRepo{}))
	refEarly := time.Date(2026, 1, 5, 10, 0, 0, 0, loc)
	n2, _ := svc2.CloseCycles(context.Background(), refEarly)
	if n2 != 0 {
		t.Errorf("antes do vencimento trimestral gerou %d, quer 0", n2)
	}
}

func TestQuarterlyPeriod(t *testing.T) {
	cases := map[time.Month]string{
		time.January: "2026-Q1", time.March: "2026-Q1",
		time.April: "2026-Q2", time.July: "2026-Q3", time.December: "2026-Q4",
	}
	for m, want := range cases {
		got := domain.QuarterlyPeriod(time.Date(2026, m, 10, 0, 0, 0, 0, time.UTC))
		if got != want {
			t.Errorf("mês %v: %q, quer %q", m, got, want)
		}
	}
}
