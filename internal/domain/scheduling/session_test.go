package scheduling

import (
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func validSession() *Session {
	start := time.Date(2026, 1, 10, 9, 0, 0, 0, clock.Location())
	return &Session{
		PatientID: "pat-1",
		Modality:  ModalityOnline,
		StartsAt:  start,
		EndsAt:    start.Add(time.Hour),
		Status:    SessionAgendada,
	}
}

func TestSessionValidate(t *testing.T) {
	if err := validSession().Validate(); err != nil {
		t.Fatalf("sessão válida: %v", err)
	}
	cases := map[string]func(*Session){
		"sem paciente":        func(s *Session) { s.PatientID = "" },
		"modalidade inválida": func(s *Session) { s.Modality = "x" },
		"horário zero":        func(s *Session) { s.StartsAt = time.Time{} },
		"fim <= início":       func(s *Session) { s.EndsAt = s.StartsAt },
		"status inválido":     func(s *Session) { s.Status = "x" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			s := validSession()
			mut(s)
			if err := s.Validate(); err == nil {
				t.Fatalf("esperava erro para %q", name)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	cases := []struct {
		cur, next string
		want      bool
	}{
		{SessionSolicitada, SessionAgendada, true},
		{SessionAgendada, SessionConfirmada, true},
		{SessionAgendada, SessionRealizada, true},
		{SessionConfirmada, SessionRealizada, true},
		{SessionRealizada, SessionCancelada, false}, // terminal
		{SessionCancelada, SessionAgendada, false},  // terminal
		{SessionFalta, SessionRealizada, false},     // terminal
	}
	for _, c := range cases {
		s := &Session{Status: c.cur}
		if got := s.CanTransition(c.next); got != c.want {
			t.Errorf("%s->%s = %v, quer %v", c.cur, c.next, got, c.want)
		}
	}
}

func TestSessionCategory(t *testing.T) {
	cases := []struct {
		s    *Session
		want string
	}{
		{&Session{Status: SessionRealizada}, CategoryRealizada},
		{&Session{Status: SessionFalta}, CategoryFalta},
		{&Session{Status: SessionCancelada, CanceledBy: CancelByPatient}, CategoryCanceladaPaciente},
		{&Session{Status: SessionCancelada, CanceledBy: CancelByTherapist}, CategoryCanceladaTerapeuta},
		{&Session{Status: SessionCancelada}, CategoryCanceladaSemAutor},
		{&Session{Status: SessionAgendada}, CategoryAgendadaConfirmada},
		{&Session{Status: SessionConfirmada}, CategoryAgendadaConfirmada},
		{&Session{Status: SessionSolicitada}, CategorySolicitada},
		{&Session{Status: "estranho"}, CategoryDesconhecida},
	}
	for _, c := range cases {
		if got := SessionCategory(c.s); got != c.want {
			t.Errorf("categoria de %q = %q, quer %q", c.s.Status, got, c.want)
		}
	}
}

func TestConfirm(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, clock.Location())
	s := validSession() // agendada
	if err := s.Confirm(now); err != nil {
		t.Fatalf("confirm válido: %v", err)
	}
	if s.Status != SessionConfirmada || !s.ConfirmedAt.Equal(now) || !s.UpdatedAt.Equal(now) {
		t.Fatalf("metadados de confirmação incorretos: %+v", s)
	}
	// Confirmar sessão terminal falha.
	term := &Session{Status: SessionRealizada}
	if err := term.Confirm(now); err == nil {
		t.Fatal("confirmar realizada deveria falhar")
	}
}

func TestCancel(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, clock.Location())
	s := validSession()
	if err := s.Cancel(CancelByPatient, "imprevisto", now); err != nil {
		t.Fatalf("cancel válido: %v", err)
	}
	if s.Status != SessionCancelada || s.CanceledBy != CancelByPatient || s.CancelReason != "imprevisto" || !s.CanceledAt.Equal(now) {
		t.Fatalf("metadados de cancelamento incorretos: %+v", s)
	}
	// Autor inválido.
	if err := validSession().Cancel("ninguem", "", now); err == nil {
		t.Fatal("autor inválido deveria falhar")
	}
	// Transição proibida (terminal).
	if err := (&Session{Status: SessionRealizada}).Cancel(CancelByPatient, "", now); err == nil {
		t.Fatal("cancelar realizada deveria falhar")
	}
}

func TestFinish(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, clock.Location())

	// bill nil => cobra.
	s := validSession()
	if err := s.Finish(nil, false, now); err != nil {
		t.Fatalf("finish válido: %v", err)
	}
	if s.Status != SessionRealizada || !s.Bill {
		t.Fatalf("bill=nil deveria cobrar: %+v", s)
	}

	// bill=false => opt-out.
	optOut := false
	s2 := validSession()
	if err := s2.Finish(&optOut, true, now); err != nil {
		t.Fatalf("finish válido: %v", err)
	}
	if s2.Bill || !s2.ConsiderCost {
		t.Fatalf("opt-out/consider_cost incorretos: %+v", s2)
	}

	// Transição proibida.
	if err := (&Session{Status: SessionCancelada}).Finish(nil, false, now); err == nil {
		t.Fatal("finish em cancelada deveria falhar")
	}
}

func TestReschedule(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, clock.Location())
	s := validSession()
	newStart := s.StartsAt.Add(24 * time.Hour)
	newEnd := newStart.Add(time.Hour)

	changed, err := s.Reschedule(newStart, newEnd, "loc-2", ModalityPresencial, now)
	if err != nil {
		t.Fatalf("reschedule válido: %v", err)
	}
	if !changed {
		t.Fatal("horário mudou, changed deveria ser true")
	}
	if !s.StartsAt.Equal(newStart) || s.LocationID != "loc-2" || s.Modality != ModalityPresencial {
		t.Fatalf("reschedule não aplicou campos: %+v", s)
	}

	// Sem mudança de horário => changed=false.
	s2 := validSession()
	changed2, err := s2.Reschedule(s2.StartsAt, s2.EndsAt, "loc-x", ModalityOnline, now)
	if err != nil {
		t.Fatalf("reschedule válido: %v", err)
	}
	if changed2 {
		t.Fatal("horário igual, changed deveria ser false")
	}

	// Terminal falha.
	if _, err := (&Session{Status: SessionRealizada}).Reschedule(newStart, newEnd, "", ModalityOnline, now); err == nil {
		t.Fatal("reschedule em terminal deveria falhar")
	}
}
