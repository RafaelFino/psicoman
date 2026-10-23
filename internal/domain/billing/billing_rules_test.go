package billing

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestBillingPeriodFor(t *testing.T) {
	loc := clock.Location()
	ref := time.Date(2026, 5, 15, 0, 0, 0, 0, loc)
	if got := BillingPeriodFor(&Plan{Type: PlanFechadoMensal}, ref); got != "2026-05" {
		t.Errorf("mensal = %q", got)
	}
	if got := BillingPeriodFor(&Plan{Type: PlanFechadoTrim}, ref); got != "2026-Q2" {
		t.Errorf("trimestral = %q", got)
	}
	if got := BillingPeriodFor(&Plan{Type: PlanPorConsulta}, ref); got != "" {
		t.Errorf("por sessão não gera período, veio %q", got)
	}
}

func TestDueDayClamp(t *testing.T) {
	if DueDay(&Plan{DueDay: 0}) != 1 {
		t.Error("due_day 0 deveria virar 1")
	}
	if DueDay(&Plan{DueDay: 31}) != 28 {
		t.Error("due_day 31 deveria virar 28")
	}
	if DueDay(&Plan{DueDay: 10}) != 10 {
		t.Error("due_day 10 deveria permanecer 10")
	}
}

func TestCycleStartMonth(t *testing.T) {
	loc := clock.Location()
	ref := time.Date(2026, 8, 20, 0, 0, 0, 0, loc) // agosto -> trimestre jul
	y, m := CycleStartMonth(&Plan{Type: PlanFechadoTrim}, ref)
	if y != 2026 || m != time.July {
		t.Errorf("trimestre de agosto começa em jul/2026, veio %d/%v", y, m)
	}
	y, m = CycleStartMonth(&Plan{Type: PlanFechadoMensal}, ref)
	if y != 2026 || m != time.August {
		t.Errorf("mensal usa o próprio mês, veio %d/%v", y, m)
	}
}

func TestDueDateForAndReached(t *testing.T) {
	loc := clock.Location()
	plan := &Plan{Type: PlanFechadoMensal, DueDay: 10}
	ref := time.Date(2026, 3, 15, 0, 0, 0, 0, loc)
	due := DueDateFor(plan, ref)
	if due.Day() != 10 || due.Month() != time.March {
		t.Fatalf("vencimento = %v, quer dia 10 de março", due)
	}
	// Exatamente no dia do vencimento => alcançado.
	if !DueReached(plan, time.Date(2026, 3, 10, 0, 0, 0, 0, loc)) {
		t.Error("no dia do vencimento deveria estar alcançado")
	}
	// Antes => não alcançado.
	if DueReached(plan, time.Date(2026, 3, 9, 0, 0, 0, 0, loc)) {
		t.Error("antes do vencimento não deveria estar alcançado")
	}
}

func TestNewSessionDebt(t *testing.T) {
	loc := clock.Location()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, loc)
	plan := &Plan{ID: "pl1", Type: PlanPorConsulta, Amount: 15000}
	d := NewSessionDebt("pat1", "sess1", plan, now)
	if d.IdempotencyKey != "session:sess1" || d.Amount != 15000 || d.Status != DebtAberto {
		t.Fatalf("débito por sessão incorreto: %+v", d)
	}
	if !d.DueDate.Equal(now.AddDate(0, 0, 7)) {
		t.Errorf("vencimento deveria ser now+7d, veio %v", d.DueDate)
	}
}

func TestNewCycleDebt(t *testing.T) {
	loc := clock.Location()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, loc)
	ref := time.Date(2026, 5, 20, 0, 0, 0, 0, loc)
	plan := &Plan{ID: "pl1", PatientID: "pat1", Type: PlanFechadoMensal, Amount: 50000, DueDay: 5}
	d := NewCycleDebt(plan, ref, now)
	if d.BillingPeriod != "2026-05" || d.IdempotencyKey != "cycle:pl1:2026-05" {
		t.Fatalf("débito de ciclo incorreto: %+v", d)
	}
	if d.Amount != 50000 || d.Status != DebtAberto {
		t.Fatalf("valor/status incorretos: %+v", d)
	}
	if d.DueDate.Day() != 5 || d.DueDate.Month() != time.May {
		t.Errorf("vencimento = %v, quer dia 5 de maio", d.DueDate)
	}
}
