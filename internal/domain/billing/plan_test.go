package billing

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func TestPlanValidate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, clock.Location())
	ok := &Plan{PatientID: "p1", Type: PlanPorConsulta, Amount: 10000, StartsAt: start}
	if err := ok.Validate(); err != nil {
		t.Fatalf("plano válido: %v", err)
	}
	cases := map[string]func(*Plan){
		"sem paciente":          func(p *Plan) { p.PatientID = "" },
		"tipo inválido":         func(p *Plan) { p.Type = "x" },
		"fechado amount<=0":     func(p *Plan) { p.Type = PlanFechadoMensal; p.Amount = 0 },
		"amount negativo":       func(p *Plan) { p.Amount = -1 },
		"due_day fora de 0..28": func(p *Plan) { p.DueDay = 29 },
		"início zero":           func(p *Plan) { p.StartsAt = time.Time{} },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			p := &Plan{PatientID: "p1", Type: PlanPorConsulta, Amount: 10000, StartsAt: start}
			mut(p)
			if err := p.Validate(); err == nil {
				t.Fatalf("esperava erro para %q", name)
			}
		})
	}
}

func TestPlanGuards(t *testing.T) {
	cases := []struct {
		typ        string
		perSession bool
		fixedCycle bool
		social     bool
	}{
		{PlanPorConsulta, true, false, false},
		{PlanPorMes, true, false, false},
		{PlanFechadoMensal, false, true, false},
		{PlanFechadoTrim, false, true, false},
		{PlanAtendimentoSoc, false, false, true},
	}
	for _, c := range cases {
		p := &Plan{Type: c.typ}
		if p.IsPerSession() != c.perSession || p.GeneratesPerSessionDebt() != c.perSession {
			t.Errorf("%s: per-session errado", c.typ)
		}
		if p.IsFixedCycle() != c.fixedCycle || p.GeneratesCycleDebt() != c.fixedCycle {
			t.Errorf("%s: fixed-cycle errado", c.typ)
		}
		if p.IsSocial() != c.social {
			t.Errorf("%s: social errado", c.typ)
		}
	}
	// Social nunca gera débito.
	soc := &Plan{Type: PlanAtendimentoSoc}
	if soc.GeneratesPerSessionDebt() || soc.GeneratesCycleDebt() {
		t.Fatal("social nunca gera débito")
	}
}

func TestPlanActiveOn(t *testing.T) {
	loc := clock.Location()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)
	p := &Plan{StartsAt: start, EndsAt: end}

	if !p.ActiveOn(time.Date(2026, 3, 1, 0, 0, 0, 0, loc)) {
		t.Error("deveria estar vigente no meio")
	}
	if p.ActiveOn(time.Date(2025, 12, 31, 0, 0, 0, 0, loc)) {
		t.Error("antes do início não é vigente")
	}
	if p.ActiveOn(time.Date(2026, 7, 1, 0, 0, 0, 0, loc)) {
		t.Error("depois do fim não é vigente")
	}
	// Sem fim: vigente para sempre após o início.
	noEnd := &Plan{StartsAt: start}
	if !noEnd.ActiveOn(time.Date(2030, 1, 1, 0, 0, 0, 0, loc)) {
		t.Error("sem fim deveria permanecer vigente")
	}
}
