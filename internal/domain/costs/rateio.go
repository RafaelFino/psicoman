package costs

import (
	"encoding/json"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PeriodBounds devolve [início, fim) do período que contém ref, conforme a
// periodicidade do custo, no fuso do projeto. Portado de service/cost.go:periodBounds.
func PeriodBounds(period string, ref time.Time) (time.Time, time.Time) {
	ref = ref.In(clock.Location())
	switch period {
	case scheduling.PeriodDiario:
		start := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(0, 0, 1)
	case scheduling.PeriodMensal:
		start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(0, 1, 0)
	case scheduling.PeriodAnual:
		start := time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(1, 0, 0)
	default:
		return ref, ref
	}
}

// RateioAmount rateia o custo do período entre count sessões: divisão inteira
// de centavos, truncando para baixo (paridade com o legado; sem distribuição de
// resto). count<1 é tratado como 1 (a própria sessão sendo finalizada).
func RateioAmount(costAmount int64, count int) int64 {
	if count < 1 {
		count = 1
	}
	return costAmount / int64(count)
}

// snapshot serializa a base do rateio em JSON auditável.
func snapshot(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// DirectCost monta o custo direto (por_sessao) de uma sessão: o valor é o custo
// cheio do local. Paridade com service/cost.go (ramo PeriodPorSessao).
func DirectCost(sessionID string, costAmount int64, costPeriod string, now time.Time) *SessionCost {
	return &SessionCost{
		ID:           ulid.New(),
		SessionID:    sessionID,
		Method:       CostMethodDireto,
		Amount:       costAmount,
		BaseSnapshot: snapshot(map[string]any{"cost_period": costPeriod, "cost_amount": costAmount}),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// RateioCost monta o custo rateado de uma sessão: o custo do período dividido
// (truncado) pelo nº de sessões realizadas no local no período [de, ate). O
// BaseSnapshot guarda a base auditável do rateio. Paridade com service/cost.go.
func RateioCost(sessionID string, costAmount int64, costPeriod string, count int, de, ate, now time.Time) *SessionCost {
	if count < 1 {
		count = 1
	}
	return &SessionCost{
		ID:        ulid.New(),
		SessionID: sessionID,
		Method:    CostMethodRateio,
		Amount:    RateioAmount(costAmount, count),
		BaseSnapshot: snapshot(map[string]any{
			"cost_period":  costPeriod,
			"cost_amount":  costAmount,
			"sessions":     count,
			"period_start": clock.Format(de),
			"period_end":   clock.Format(ate),
		}),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
