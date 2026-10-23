package scheduling

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
	ContractID    string // contrato associado (opcional, D4b.2)
	PlanID        string // plano de cobrança associado (opcional, D4b.2)
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

// --- Comportamento rico de transição de estado ---
//
// As transições que MUTAM a sessão e preenchem metadados vivem no agregado
// (OO, R4): validam a transição, aplicam os campos derivados (confirmed_at,
// canceled_by, bill, etc.) e atualizam updated_at. O serviço de app apenas
// coordena I/O (persistência), auditoria e hooks — não reimplementa a regra.

// Schedule move uma sessão solicitada para "agendada". O serviço de app faz a
// checagem de conflito e a criação do evento de Calendar ANTES de chamar.
func (s *Session) Schedule(now time.Time) error {
	if !s.CanTransition(SessionAgendada) {
		return errors.New("A sessão não pode ser agendada a partir do estado atual.")
	}
	s.Status = SessionAgendada
	s.UpdatedAt = now
	return nil
}

// Confirm move a sessão de "agendada" para "confirmada", gravando confirmed_at.
func (s *Session) Confirm(now time.Time) error {
	if !s.CanTransition(SessionConfirmada) {
		return errors.New("A sessão não pode ser confirmada a partir do estado atual.")
	}
	s.Status = SessionConfirmada
	s.ConfirmedAt = now
	s.UpdatedAt = now
	return nil
}

// Cancel move a sessão para "cancelada", exigindo o autor (paciente|terapeuta)
// e gravando o motivo opcional e canceled_at.
func (s *Session) Cancel(by, reason string, now time.Time) error {
	if !ValidCancelBy(by) {
		return errors.New("Informe quem cancelou: paciente ou terapeuta.")
	}
	if !s.CanTransition(SessionCancelada) {
		return errors.New("A sessão não pode ser cancelada a partir do estado atual.")
	}
	s.Status = SessionCancelada
	s.CanceledBy = by
	s.CancelReason = reason
	s.CanceledAt = now
	s.UpdatedAt = now
	return nil
}

// MarkNoShow move a sessão para "falta".
func (s *Session) MarkNoShow(now time.Time) error {
	if !s.CanTransition(SessionFalta) {
		return errors.New("Transição de estado não permitida.")
	}
	s.Status = SessionFalta
	s.UpdatedAt = now
	return nil
}

// Finish move a sessão para "realizada" e grava os flags de cobrança/custo. O
// padrão de uma sessão realizada é gerar cobrança (bill nil = true); o
// terapeuta só desmarca (bill=false) para opt-out explícito (R1.3).
func (s *Session) Finish(bill *bool, considerCost bool, now time.Time) error {
	if !s.CanTransition(SessionRealizada) {
		return errors.New("A sessão não pode ser finalizada a partir do estado atual.")
	}
	s.Status = SessionRealizada
	s.Bill = bill == nil || *bill
	s.ConsiderCost = considerCost
	s.UpdatedAt = now
	return nil
}

// Reschedule altera data/hora, local e modalidade de uma sessão não terminal,
// sem trocar o paciente. Devolve changed=true quando o horário efetivamente
// mudou (base para o audit session.reschedule, feito no serviço de app). Não
// reconcilia o Google Calendar aqui (ver docs/decisions.md 2026-10-12).
func (s *Session) Reschedule(start, end time.Time, locationID, modality string, now time.Time) (changed bool, err error) {
	if s.IsTerminal() {
		return false, errors.New("Esta sessão já foi concluída e não pode ser reagendada.")
	}
	fromStart, fromEnd := s.StartsAt, s.EndsAt
	s.LocationID = locationID
	s.Modality = modality
	s.StartsAt = start
	s.EndsAt = end
	s.UpdatedAt = now
	if err := s.Validate(); err != nil {
		return false, err
	}
	changed = !fromStart.Equal(start) || !fromEnd.Equal(end)
	return changed, nil
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
