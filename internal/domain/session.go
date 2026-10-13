package domain

import (
	"errors"
	"time"
)

// Estados do ciclo de vida da sessão (requirements §3.3; patient-timeline R6.1).
const (
	SessionSolicitada = "solicitada"
	SessionAgendada   = "agendada"
	SessionConfirmada = "confirmada"
	SessionRealizada  = "realizada"
	SessionCancelada  = "cancelada"
	SessionFalta      = "falta"
)

// Autor do cancelamento (patient-timeline R6.2).
const (
	CancelByPatient   = "paciente"
	CancelByTherapist = "terapeuta"
)

// ValidCancelBy indica se s é um autor de cancelamento conhecido.
func ValidCancelBy(s string) bool {
	return s == CancelByPatient || s == CancelByTherapist
}

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
	// Metadados ortogonais de status (Opção B, patient-timeline R6).
	ConfirmedAt  time.Time // zero = não confirmada
	CanceledBy   string    // paciente|terapeuta (vazio quando não cancelada)
	CancelReason string    // texto livre PT-BR, opcional
	CanceledAt   time.Time // zero = não cancelada
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// transitions define as transições válidas do ciclo de vida.
var transitions = map[string]map[string]bool{
	SessionSolicitada: {SessionAgendada: true, SessionCancelada: true},
	SessionAgendada:   {SessionConfirmada: true, SessionRealizada: true, SessionCancelada: true, SessionFalta: true},
	SessionConfirmada: {SessionRealizada: true, SessionCancelada: true, SessionFalta: true},
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

// Categorias de relatório derivadas de status + canceled_by (patient-timeline
// R6.4). Base dos relatórios/métricas (R7, R6.4–R6.5).
const (
	CategoryRealizada          = "realizadas"
	CategoryFalta              = "faltas"
	CategoryCanceladaPaciente  = "canceladas_paciente"
	CategoryCanceladaTerapeuta = "canceladas_terapeuta"
	CategoryCanceladaSemAutor  = "canceladas_sem_autor"
	CategoryAgendadaConfirmada = "agendadas_confirmadas"
	CategorySolicitada         = "solicitadas"
	CategoryDesconhecida       = "desconhecida"
)

// SessionCategory devolve a categoria de relatório de uma sessão, combinando o
// status com o autor do cancelamento. Função pura, reutilizada pelos
// relatórios de paciente e de local.
func SessionCategory(s *Session) string {
	switch s.Status {
	case SessionRealizada:
		return CategoryRealizada
	case SessionFalta:
		return CategoryFalta
	case SessionCancelada:
		switch s.CanceledBy {
		case CancelByPatient:
			return CategoryCanceladaPaciente
		case CancelByTherapist:
			return CategoryCanceladaTerapeuta
		default:
			return CategoryCanceladaSemAutor
		}
	case SessionAgendada, SessionConfirmada:
		return CategoryAgendadaConfirmada
	case SessionSolicitada:
		return CategorySolicitada
	default:
		return CategoryDesconhecida
	}
}
