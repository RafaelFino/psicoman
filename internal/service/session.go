package service

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// SessionRepository persiste sessões.
type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	Update(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, id string) (*domain.Session, error)
	List(ctx context.Context) ([]*domain.Session, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.Session, error)
	// ListByRange devolve as sessões cujo início está em [from, to).
	ListByRange(ctx context.Context, from, to time.Time) ([]*domain.Session, error)
	// ListByLocationRange devolve as sessões de um local cujo início está em
	// [from, to). Limites zero = sem limite (histórico completo do local).
	ListByLocationRange(ctx context.Context, locationID string, from, to time.Time) ([]*domain.Session, error)
	SoftDelete(ctx context.Context, id string) error
}

// ConflictChecker verifica conflito de agenda antes de confirmar uma sessão.
// Implementação real (freebusy do Calendar) chega na Task 15; até lá, o
// noopConflictChecker sempre libera.
type ConflictChecker interface {
	HasConflict(ctx context.Context, startsAt, endsAt time.Time) (bool, error)
}

// noopConflictChecker nunca acusa conflito (usado antes da integração Google).
type noopConflictChecker struct{}

func (noopConflictChecker) HasConflict(context.Context, time.Time, time.Time) (bool, error) {
	return false, nil
}

// calendarConflictChecker adapta um CalendarClient (freebusy) ao ConflictChecker.
type calendarConflictChecker struct{ cal CalendarClient }

// NewCalendarConflictChecker cria um ConflictChecker baseado no freebusy do Calendar.
func NewCalendarConflictChecker(cal CalendarClient) ConflictChecker {
	return calendarConflictChecker{cal: cal}
}

func (c calendarConflictChecker) HasConflict(ctx context.Context, startsAt, endsAt time.Time) (bool, error) {
	return c.cal.FreeBusy(ctx, startsAt, endsAt)
}

// SessionFinishedHook é chamado ao finalizar uma sessão (status=realizada),
// permitindo que o financeiro/custos reajam. Injetado pelas Tasks 9/13.
type SessionFinishedHook interface {
	OnSessionFinished(ctx context.Context, s *domain.Session) error
}

// SessionCalendar cria/remove o evento no Google Calendar ao efetivar a sessão
// (Task 15). Opcional: sem Google configurado, a sessão é agendada sem evento.
type SessionCalendar interface {
	CreateEvent(ctx context.Context, event CalendarEvent) (*CalendarEventResult, error)
}

// SessionService orquestra o ciclo de vida das sessões.
type SessionService struct {
	repo      SessionRepository
	patients  PatientRepository
	conflict  ConflictChecker
	calendar  SessionCalendar
	reminders []int
	hooks     []SessionFinishedHook
	audit     *AuditService
	clock     clock.Clock
}

// SetAudit injeta o serviço de auditoria para registrar operações sensíveis de
// sessão (confirmação, cancelamento, remarcação). Best-effort: falha de audit
// não derruba a operação.
func (s *SessionService) SetAudit(a *AuditService) { s.audit = a }

// NewSessionService cria o serviço. Sem Google, usa o checker no-op.
func NewSessionService(repo SessionRepository, patients PatientRepository) *SessionService {
	return &SessionService{
		repo:     repo,
		patients: patients,
		conflict: noopConflictChecker{},
		clock:    clock.System{},
	}
}

// SetConflictChecker injeta o verificador de conflito (Task 15).
func (s *SessionService) SetConflictChecker(c ConflictChecker) {
	if c != nil {
		s.conflict = c
	}
}

// SetCalendar injeta o cliente de calendário e os reminders configurados
// (Task 15). Ao agendar, cria evento + Meet + convidado.
func (s *SessionService) SetCalendar(cal SessionCalendar, reminderMinutes []int) {
	s.calendar = cal
	s.reminders = reminderMinutes
}

// AddFinishedHook registra um hook de finalização (Tasks 9/13).
func (s *SessionService) AddFinishedHook(h SessionFinishedHook) {
	s.hooks = append(s.hooks, h)
}

// CreateInput são os dados de criação de uma sessão.
type SessionCreateInput struct {
	PatientID  string
	LocationID string
	RequestID  string
	Modality   string
	StartsAt   time.Time
	EndsAt     time.Time
	// Status inicial: agendada (direto pelo terapeuta) ou solicitada (portal).
	Status string
}

// Create valida e cria uma sessão. Ao criar já como "agendada", verifica
// conflito de agenda (no-op até a Task 15).
func (s *SessionService) Create(ctx context.Context, in SessionCreateInput) (*domain.Session, error) {
	if _, err := s.patients.Get(ctx, in.PatientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	status := in.Status
	if status == "" {
		status = domain.SessionAgendada
	}
	sess := &domain.Session{
		ID:         ulid.New(),
		PatientID:  in.PatientID,
		LocationID: in.LocationID,
		RequestID:  in.RequestID,
		Modality:   in.Modality,
		StartsAt:   in.StartsAt,
		EndsAt:     in.EndsAt,
		Status:     status,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := sess.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if status == domain.SessionAgendada {
		if err := s.ensureNoConflict(ctx, sess); err != nil {
			return nil, err
		}
		if err := s.createCalendarEvent(ctx, sess); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// SessionUpdateInput são os campos editáveis de uma sessão. Nunca inclui o
// paciente: uma sessão pertence a um paciente e trocá-lo é criar outra sessão.
type SessionUpdateInput struct {
	LocationID string
	Modality   string
	StartsAt   time.Time
	EndsAt     time.Time
}

// Update altera os dados de agendamento (data/hora, local e modalidade) de uma
// sessão existente, sem trocar o paciente. A edição de horário/local é bloqueada
// em estados terminais (realizada/cancelada/falta); o registro clínico (notas
// vinculadas) segue por outra rota e continua permitido. Não reconcilia o
// Google Calendar nesta onda (ver docs/decisions.md 2026-10-12): a UI avisa o
// terapeuta para ajustar o convite manualmente quando a sessão tem evento.
func (s *SessionService) Update(ctx context.Context, id string, in SessionUpdateInput) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.IsTerminal() {
		return nil, NewValidation("Esta sessão já foi concluída e não pode ser reagendada.")
	}
	// Captura o horário anterior para registrar a remarcação (R4.3).
	fromStart, fromEnd := sess.StartsAt, sess.EndsAt
	sess.LocationID = in.LocationID
	sess.Modality = in.Modality
	sess.StartsAt = in.StartsAt
	sess.EndsAt = in.EndsAt
	sess.UpdatedAt = s.clock.Now()
	if err := sess.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if sess.Status == domain.SessionAgendada {
		if err := s.ensureNoConflict(ctx, sess); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	// Remarcação: quando o horário muda numa sessão agendada/confirmada, grava
	// audit consultável (session.reschedule), sem conteúdo clínico (R4.3).
	rescheduled := !fromStart.Equal(sess.StartsAt) || !fromEnd.Equal(sess.EndsAt)
	if rescheduled && s.audit != nil &&
		(sess.Status == domain.SessionAgendada || sess.Status == domain.SessionConfirmada) {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionSessionReschedule, "session", sess.ID,
			map[string]any{
				"session_id": sess.ID,
				"from":       clock.Format(fromStart),
				"to":         clock.Format(sess.StartsAt),
			})
	}
	return sess, nil
}

// Schedule confirma uma sessão solicitada, movendo-a para "agendada" após
// checar conflito de agenda.
func (s *SessionService) Schedule(ctx context.Context, id string) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(domain.SessionAgendada) {
		return nil, NewValidation("A sessão não pode ser agendada a partir do estado atual.")
	}
	if err := s.ensureNoConflict(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.createCalendarEvent(ctx, sess); err != nil {
		return nil, err
	}
	return s.transition(ctx, sess, domain.SessionAgendada)
}

// createCalendarEvent cria o evento no Calendar (com Meet + convidado +
// reminders) e grava o event_id/meet_url na sessão. No-op se sem Calendar.
func (s *SessionService) createCalendarEvent(ctx context.Context, sess *domain.Session) error {
	if s.calendar == nil {
		return nil
	}
	patient, err := s.patients.Get(ctx, sess.PatientID)
	if err != nil {
		return err
	}
	res, err := s.calendar.CreateEvent(ctx, CalendarEvent{
		Summary:         "Sessão — " + patient.Name,
		Description:     "Atendimento psicológico.",
		StartsAt:        sess.StartsAt,
		EndsAt:          sess.EndsAt,
		AttendeeEmail:   patient.Email,
		ReminderMinutes: s.reminders,
		WithMeet:        true,
	})
	if err != nil {
		return err
	}
	sess.GoogleEventID = res.EventID
	sess.MeetURL = res.MeetURL
	return nil
}

// Confirm move a sessão de "agendada" para "confirmada", gravando confirmed_at.
// A confirmação é opcional no fluxo (uma sessão agendada ainda pode ir direto a
// realizada/falta/cancelada). Audita session.confirm (sem conteúdo clínico).
func (s *SessionService) Confirm(ctx context.Context, id string) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(domain.SessionConfirmada) {
		return nil, NewValidation("A sessão não pode ser confirmada a partir do estado atual.")
	}
	sess.Status = domain.SessionConfirmada
	sess.ConfirmedAt = s.clock.Now()
	sess.UpdatedAt = sess.ConfirmedAt
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionSessionConfirm, "session", sess.ID,
			map[string]any{"confirmed_at": clock.Format(sess.ConfirmedAt)})
	}
	return sess, nil
}

// Cancel move a sessão para "cancelada", exigindo o autor (paciente|terapeuta)
// e gravando o motivo opcional e canceled_at. Audita session.cancel com
// metadata {canceled_by} — o motivo NÃO entra em metadata (coerente com §4.1).
func (s *SessionService) Cancel(ctx context.Context, id, by, reason string) (*domain.Session, error) {
	if !domain.ValidCancelBy(by) {
		return nil, NewValidation("Informe quem cancelou: paciente ou terapeuta.")
	}
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(domain.SessionCancelada) {
		return nil, NewValidation("A sessão não pode ser cancelada a partir do estado atual.")
	}
	now := s.clock.Now()
	sess.Status = domain.SessionCancelada
	sess.CanceledBy = by
	sess.CancelReason = reason
	sess.CanceledAt = now
	sess.UpdatedAt = now
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionSessionCancel, "session", sess.ID,
			map[string]any{"canceled_by": by})
	}
	return sess, nil
}

// MarkNoShow move a sessão para "falta".
func (s *SessionService) MarkNoShow(ctx context.Context, id string) (*domain.Session, error) {
	return s.simpleTransition(ctx, id, domain.SessionFalta)
}

// FinishInput são os flags explícitos da finalização. Bill é opcional (ponteiro)
// para distinguir "não informado" (padrão = cobrar) de opt-out explícito
// (false) — ver patient-timeline R1.3.
type FinishInput struct {
	Bill         *bool
	ConsiderCost bool
}

// Finish move a sessão para "realizada", grava os flags e dispara os hooks
// (financeiro/custos). O padrão de uma sessão realizada é gerar cobrança
// (Bill = true); o terapeuta só desmarca para opt-out explícito.
func (s *SessionService) Finish(ctx context.Context, id string, in FinishInput) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(domain.SessionRealizada) {
		return nil, NewValidation("A sessão não pode ser finalizada a partir do estado atual.")
	}
	sess.Status = domain.SessionRealizada
	// Padrão: cobrar. Bill == nil (não informado) ⇒ true; false ⇒ opt-out.
	sess.Bill = in.Bill == nil || *in.Bill
	sess.ConsiderCost = in.ConsiderCost
	sess.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	for _, h := range s.hooks {
		if err := h.OnSessionFinished(ctx, sess); err != nil {
			return nil, err
		}
	}
	return sess, nil
}

// Get devolve uma sessão por id.
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, error) {
	return s.repo.Get(ctx, id)
}

// List devolve todas as sessões.
func (s *SessionService) List(ctx context.Context) ([]*domain.Session, error) {
	return s.repo.List(ctx)
}

// ListByPatient devolve as sessões de um paciente.
func (s *SessionService) ListByPatient(ctx context.Context, patientID string) ([]*domain.Session, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// ListByRange devolve as sessões cujo início está no intervalo [from, to).
// Serve às visões de agenda (semana/mês) sem trazer o histórico inteiro.
func (s *SessionService) ListByRange(ctx context.Context, from, to time.Time) ([]*domain.Session, error) {
	return s.repo.ListByRange(ctx, from, to)
}

func (s *SessionService) simpleTransition(ctx context.Context, id, next string) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(next) {
		return nil, NewValidation("Transição de estado não permitida.")
	}
	return s.transition(ctx, sess, next)
}

func (s *SessionService) transition(ctx context.Context, sess *domain.Session, next string) (*domain.Session, error) {
	sess.Status = next
	sess.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *SessionService) ensureNoConflict(ctx context.Context, sess *domain.Session) error {
	conflict, err := s.conflict.HasConflict(ctx, sess.StartsAt, sess.EndsAt)
	if err != nil {
		return err
	}
	if conflict {
		return NewConflict("Já existe um compromisso na agenda nesse horário.")
	}
	return nil
}
