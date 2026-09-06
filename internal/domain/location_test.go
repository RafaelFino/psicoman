package domain

import "testing"

func TestLocationValidate(t *testing.T) {
	cases := []struct {
		name string
		loc  Location
		ok   bool
	}{
		{"válido presencial mensal", Location{Name: "C", Modality: "presencial", CostPeriod: "mensal", CostAmount: 1000}, true},
		{"válido online por_sessao", Location{Name: "O", Modality: "online", CostPeriod: "por_sessao"}, true},
		{"sem nome", Location{Modality: "online", CostPeriod: "anual"}, false},
		{"modalidade inválida", Location{Name: "X", Modality: "hibrido", CostPeriod: "anual"}, false},
		{"periodo inválido", Location{Name: "X", Modality: "online", CostPeriod: "semanal"}, false},
		{"custo negativo", Location{Name: "X", Modality: "online", CostPeriod: "diario", CostAmount: -1}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.loc.Validate()
			if c.ok && err != nil {
				t.Errorf("esperava válido, veio: %v", err)
			}
			if !c.ok && err == nil {
				t.Error("esperava erro")
			}
		})
	}
}

func TestAvailabilityValidate(t *testing.T) {
	cases := []struct {
		name string
		a    Availability
		ok   bool
	}{
		{"válido", Availability{Weekday: 1, StartTime: "09:00", EndTime: "12:00", Capacity: 1}, true},
		{"weekday inválido", Availability{Weekday: 7, StartTime: "09:00", EndTime: "12:00", Capacity: 1}, false},
		{"horário formato ruim", Availability{Weekday: 1, StartTime: "9h", EndTime: "12:00", Capacity: 1}, false},
		{"fim antes do início", Availability{Weekday: 1, StartTime: "15:00", EndTime: "10:00", Capacity: 1}, false},
		{"capacidade zero", Availability{Weekday: 1, StartTime: "09:00", EndTime: "12:00", Capacity: 0}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.a.Validate()
			if c.ok && err != nil {
				t.Errorf("esperava válido, veio: %v", err)
			}
			if !c.ok && err == nil {
				t.Error("esperava erro")
			}
		})
	}
}
