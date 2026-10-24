package costs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestRateioAmount(t *testing.T) {
	cases := []struct {
		cost  int64
		count int
		want  int64
	}{
		{10000, 4, 2500},  // divisão exata
		{10000, 3, 3333},  // trunca (resto descartado)
		{10000, 0, 10000}, // count 0 => 1
		{10000, 1, 10000}, // período com 1 sessão (a própria)
	}
	for _, c := range cases {
		if got := RateioAmount(c.cost, c.count); got != c.want {
			t.Errorf("RateioAmount(%d,%d) = %d, quer %d", c.cost, c.count, got, c.want)
		}
	}
}

func TestPeriodBounds(t *testing.T) {
	loc := clock.Location()
	ref := time.Date(2026, 3, 15, 14, 30, 0, 0, loc)

	de, ate := PeriodBounds(scheduling.PeriodDiario, ref)
	if de.Day() != 15 || ate.Day() != 16 {
		t.Errorf("diário = [%v,%v)", de, ate)
	}
	de, ate = PeriodBounds(scheduling.PeriodMensal, ref)
	if de.Day() != 1 || de.Month() != time.March || ate.Month() != time.April {
		t.Errorf("mensal = [%v,%v)", de, ate)
	}
	de, ate = PeriodBounds(scheduling.PeriodAnual, ref)
	if de.Month() != time.January || de.Year() != 2026 || ate.Year() != 2027 {
		t.Errorf("anual = [%v,%v)", de, ate)
	}
}

func TestDirectCost(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, clock.Location())
	sc := DirectCost("sess1", 8000, scheduling.PeriodPorSessao, now)
	if sc.Method != CostMethodDireto || sc.Amount != 8000 {
		t.Fatalf("custo direto incorreto: %+v", sc)
	}
	var base map[string]any
	if err := json.Unmarshal([]byte(sc.BaseSnapshot), &base); err != nil {
		t.Fatalf("snapshot inválido: %v", err)
	}
	if base["cost_period"] != scheduling.PeriodPorSessao {
		t.Errorf("snapshot sem cost_period: %v", base)
	}
}

func TestRateioCost(t *testing.T) {
	loc := clock.Location()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, loc)
	de := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	ate := de.AddDate(0, 1, 0)
	sc := RateioCost("sess1", 9000, scheduling.PeriodMensal, 3, de, ate, now)
	if sc.Method != CostMethodRateio || sc.Amount != 3000 {
		t.Fatalf("custo rateado incorreto: %+v", sc)
	}
	var base map[string]any
	if err := json.Unmarshal([]byte(sc.BaseSnapshot), &base); err != nil {
		t.Fatalf("snapshot inválido: %v", err)
	}
	if int(base["sessions"].(float64)) != 3 {
		t.Errorf("snapshot sessions errado: %v", base)
	}

	// Período vazio: count<1 tratado como 1 => custo cheio.
	empty := RateioCost("sess2", 9000, scheduling.PeriodMensal, 0, de, ate, now)
	if empty.Amount != 9000 {
		t.Errorf("período vazio deveria render custo cheio, veio %d", empty.Amount)
	}
}
