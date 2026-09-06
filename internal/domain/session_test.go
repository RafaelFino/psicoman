package domain

import (
	"testing"
	"time"
)

func TestSessionTransitions(t *testing.T) {
	cases := []struct {
		from string
		to   string
		ok   bool
	}{
		{SessionSolicitada, SessionAgendada, true},
		{SessionSolicitada, SessionCancelada, true},
		{SessionSolicitada, SessionRealizada, false},
		{SessionSolicitada, SessionFalta, false},
		{SessionAgendada, SessionRealizada, true},
		{SessionAgendada, SessionCancelada, true},
		{SessionAgendada, SessionFalta, true},
		{SessionAgendada, SessionSolicitada, false},
		{SessionRealizada, SessionCancelada, false},
		{SessionCancelada, SessionAgendada, false},
		{SessionFalta, SessionRealizada, false},
	}
	for _, c := range cases {
		s := &Session{Status: c.from}
		if got := s.CanTransition(c.to); got != c.ok {
			t.Errorf("%s→%s: CanTransition=%v, quer %v", c.from, c.to, got, c.ok)
		}
	}
}

func TestSessionIsTerminal(t *testing.T) {
	for _, st := range []string{SessionRealizada, SessionCancelada, SessionFalta} {
		if !(&Session{Status: st}).IsTerminal() {
			t.Errorf("%s deveria ser terminal", st)
		}
	}
	for _, st := range []string{SessionSolicitada, SessionAgendada} {
		if (&Session{Status: st}).IsTerminal() {
			t.Errorf("%s não deveria ser terminal", st)
		}
	}
}

func TestSessionValidate(t *testing.T) {
	start := time.Now()
	valid := &Session{
		PatientID: "p1", Modality: ModalityOnline,
		StartsAt: start, EndsAt: start.Add(time.Hour), Status: SessionAgendada,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("esperava válido: %v", err)
	}

	badTime := *valid
	badTime.EndsAt = start.Add(-time.Hour)
	if err := badTime.Validate(); err == nil {
		t.Error("fim antes do início deveria falhar")
	}

	noPatient := *valid
	noPatient.PatientID = ""
	if err := noPatient.Validate(); err == nil {
		t.Error("sem paciente deveria falhar")
	}
}
