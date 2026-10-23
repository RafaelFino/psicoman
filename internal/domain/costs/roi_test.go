package costs

import (
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

func TestPeriodizedCost(t *testing.T) {
	cases := []struct {
		period string
		amount int64
		days   float64
		want   int64
	}{
		{scheduling.PeriodDiario, 1000, 10, 10000},   // ×days
		{scheduling.PeriodMensal, 3000, 30, 3000},    // ÷30 × 30
		{scheduling.PeriodMensal, 3000, 15, 1500},    // ÷30 × 15
		{scheduling.PeriodAnual, 36500, 365, 36500},  // ÷365 × 365
		{scheduling.PeriodPorSessao, 5000, 99, 5000}, // não periodiza
	}
	for _, c := range cases {
		it := &CostItem{Period: c.period, Amount: c.amount}
		if got := PeriodizedCost(it, c.days); got != c.want {
			t.Errorf("PeriodizedCost(%s,%d,%v) = %d, quer %d", c.period, c.amount, c.days, got, c.want)
		}
	}
}
