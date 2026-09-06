package domain

import (
	"errors"
	"time"
)

// Estados do ciclo de vida da sessão (requirements §3.3).
const (
	SessionSolicitada = "solicitada"
	SessionAgendada   = "agendada"
	SessionRealizada  = "realizada"
	SessionCancelada  = "cancelada"
	SessionFalta      = "falta"
)

// Session é uma sessão de atendimento.
type Session struct {
	ID            string
	PatientID     string
	LocationID    string
	RequestID     string // pedido de agendamento que a originou (opcional)
	Modality      string
	StartsAt      time.Time
	EndsAt        time.Time
	Status        string
	Bill          bool // haverá cobrança
	ConsiderCost  bool // considerar custos
	GoogleEventID string
	MeetURL       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// transitions define as transições válidas do ciclo de vida.
var transitions = map[string]map[string]bool{
	SessionSolicitada: {SessionAgendada: true, SessionCancelada: true},
	SessionAgendada:   {SessionRealizada: true, SessionCancelada: true, SessionFalta: true},
	SessionRealizada:  {},
	SessionCancelada:  {},
	SessionFalta:      {},
}

// ValidStatus indica se s é um estado conhecido.
func ValidStatus(s string) bool {
	_, ok := transitions[s]
	return ok
}

// CanTransition indica se a transição do estado atual para next é permitida.
func (s *Session) CanTransition(next string) bool {
	allowed, ok := transitions[s.Status]
	if !ok {
		return false
	}
	return allowed[next]
}

// IsTerminal indica se o estado atual é final.
func (s *Session) IsTerminal() bool {
	switch s.Status {
	case SessionRealizada, SessionCancelada, SessionFalta:
		return true
	}
	return false
}

// Validate verifica as invariantes de shape da sessão.
func (s *Session) Validate() error {
	if s.PatientID == "" {
		return errors.New("A sessão precisa de um paciente.")
	}
	if !ValidModality(s.Modality) {
		return errors.New("A modalidade deve ser 'presencial' ou 'online'.")
	}
	if s.StartsAt.IsZero() || s.EndsAt.IsZero() {
		return errors.New("Os horários de início e fim são obrigatórios.")
	}
	if !s.EndsAt.After(s.StartsAt) {
		return errors.New("O horário de término deve ser posterior ao de início.")
	}
	if !ValidStatus(s.Status) {
		return errors.New("Estado de sessão inválido.")
	}
	return nil
}
