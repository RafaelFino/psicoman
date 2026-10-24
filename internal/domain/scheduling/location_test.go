package scheduling

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func mustTime() time.Time {
	return time.Date(2026, 2, 1, 9, 0, 0, 0, clock.Location())
}

func TestLocationValidate(t *testing.T) {
	ok := &Location{Name: "Sala 1", Modality: ModalityPresencial, CostPeriod: PeriodMensal, CostAmount: 50000}
	if err := ok.Validate(); err != nil {
		t.Fatalf("local válido: %v", err)
	}
	cases := map[string]func(*Location){
		"nome vazio":          func(l *Location) { l.Name = " " },
		"modalidade inválida": func(l *Location) { l.Modality = "x" },
		"período inválido":    func(l *Location) { l.CostPeriod = "x" },
		"custo negativo":      func(l *Location) { l.CostAmount = -1 },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			l := &Location{Name: "Sala 1", Modality: ModalityPresencial, CostPeriod: PeriodMensal}
			mut(l)
			if err := l.Validate(); err == nil {
				t.Fatalf("esperava erro para %q", name)
			}
		})
	}
}

func TestAvailabilityValidate(t *testing.T) {
	ok := &Availability{Weekday: 1, StartTime: "08:00", EndTime: "12:00", Capacity: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("disponibilidade válida: %v", err)
	}
	cases := map[string]func(*Availability){
		"weekday fora":   func(a *Availability) { a.Weekday = 7 },
		"HH:MM inválido": func(a *Availability) { a.StartTime = "25:00" },
		"start >= end":   func(a *Availability) { a.StartTime = "12:00"; a.EndTime = "12:00" },
		"capacidade < 1": func(a *Availability) { a.Capacity = 0 },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			a := &Availability{Weekday: 1, StartTime: "08:00", EndTime: "12:00", Capacity: 1}
			mut(a)
			if err := a.Validate(); err == nil {
				t.Fatalf("esperava erro para %q", name)
			}
		})
	}
}

func TestAppointmentValidate(t *testing.T) {
	start := mustTime()
	ok := &AppointmentRequest{PatientID: "p1", SlotStart: start, SlotEnd: start.Add(time.Hour)}
	if err := ok.Validate(); err != nil {
		t.Fatalf("pedido válido: %v", err)
	}
	if err := (&AppointmentRequest{SlotStart: start, SlotEnd: start.Add(time.Hour)}).Validate(); err == nil {
		t.Fatal("sem paciente deveria falhar")
	}
	if err := (&AppointmentRequest{PatientID: "p1"}).Validate(); err == nil {
		t.Fatal("sem slots deveria falhar")
	}
	if err := (&AppointmentRequest{PatientID: "p1", SlotStart: start, SlotEnd: start}).Validate(); err == nil {
		t.Fatal("fim <= início deveria falhar")
	}
}
