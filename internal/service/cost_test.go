package service

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestPeriodBounds(t *testing.T) {
	ref := time.Date(2026, 3, 15, 14, 30, 0, 0, clock.Location())

	de, ate := periodBounds(domain.PeriodMensal, ref)
	if de.Month() != time.March || de.Day() != 1 {
		t.Errorf("início mensal = %v, quer 2026-03-01", de)
	}
	if ate.Month() != time.April || ate.Day() != 1 {
		t.Errorf("fim mensal = %v, quer 2026-04-01", ate)
	}

	de, ate = periodBounds(domain.PeriodDiario, ref)
	if de.Day() != 15 || ate.Day() != 16 {
		t.Errorf("bounds diários = [%v, %v)", de, ate)
	}

	de, ate = periodBounds(domain.PeriodAnual, ref)
	if de.Year() != 2026 || de.Month() != time.January || ate.Year() != 2027 {
		t.Errorf("bounds anuais = [%v, %v)", de, ate)
	}
}

func TestPeriodizedCost(t *testing.T) {
	// Custo mensal de R$ 3000 (300000 centavos) por 30 dias = R$ 3000.
	monthly := &domain.CostItem{Amount: 300000, Period: domain.PeriodMensal}
	if got := periodizedCost(monthly, 30); got != 300000 {
		t.Errorf("periodizedCost mensal 30d = %d, quer 300000", got)
	}
	// Custo anual de R$ 1200 por 365 dias = R$ 1200.
	annual := &domain.CostItem{Amount: 120000, Period: domain.PeriodAnual}
	if got := periodizedCost(annual, 365); got != 120000 {
		t.Errorf("periodizedCost anual 365d = %d, quer 120000", got)
	}
}
