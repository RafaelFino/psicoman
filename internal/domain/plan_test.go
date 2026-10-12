package domain

import (
	"testing"
	"time"
)

func TestPlanValidateDueDay(t *testing.T) {
	base := func(dueDay int) *Plan {
		return &Plan{
			PatientID: "p1",
			Type:      PlanFechadoMensal,
			Amount:    50000,
			DueDay:    dueDay,
			StartsAt:  time.Now(),
		}
	}
	cases := []struct {
		name    string
		dueDay  int
		wantErr bool
	}{
		{"zero (não aplicável)", 0, false},
		{"dia 1", 1, false},
		{"dia 28", 28, false},
		{"dia 29 inválido", 29, true},
		{"negativo inválido", -1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := base(c.dueDay).Validate()
			if c.wantErr && err == nil {
				t.Errorf("due_day=%d: esperava erro, veio nil", c.dueDay)
			}
			if !c.wantErr && err != nil {
				t.Errorf("due_day=%d: não esperava erro, veio %v", c.dueDay, err)
			}
		})
	}
}
