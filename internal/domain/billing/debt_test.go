package billing

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestIdempotencyKeys(t *testing.T) {
	if got := SessionIdempotencyKey("s1"); got != "session:s1" {
		t.Errorf("session key = %q", got)
	}
	if got := CycleIdempotencyKey("pl1", "2026-01"); got != "cycle:pl1:2026-01" {
		t.Errorf("cycle key = %q", got)
	}
}

func TestPeriods(t *testing.T) {
	loc := clock.Location()
	ref := time.Date(2026, 5, 15, 10, 0, 0, 0, loc)
	if got := MonthlyPeriod(ref); got != "2026-05" {
		t.Errorf("monthly = %q", got)
	}
	// Limites de trimestre.
	cases := map[time.Month]string{
		time.January: "2026-Q1", time.April: "2026-Q2",
		time.July: "2026-Q3", time.October: "2026-Q4",
	}
	for m, want := range cases {
		q := QuarterlyPeriod(time.Date(2026, m, 1, 0, 0, 0, 0, loc))
		if q != want {
			t.Errorf("trimestre mês %v = %q, quer %q", m, q, want)
		}
	}
}

func TestApplyPayment(t *testing.T) {
	d := &Debt{Amount: 10000}
	if st := d.ApplyPayment(5000); st != DebtParcial || d.Status != DebtParcial {
		t.Fatalf("pagamento parcial = %q", st)
	}
	if st := d.ApplyPayment(10000); st != DebtPago || d.Status != DebtPago {
		t.Fatalf("pagamento total = %q", st)
	}
	if st := d.ApplyPayment(12000); st != DebtPago {
		t.Fatalf("pagamento acima = %q (deveria pago)", st)
	}
}
