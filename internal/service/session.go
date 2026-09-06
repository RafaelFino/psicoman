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
	clock     clock.Clock
}

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

// Cancel move a sessão para "cancelada".
func (s *SessionService) Cancel(ctx context.Context, id string) (*domain.Session, error) {
	return s.simpleTransition(ctx, id, domain.SessionCancelada)
}

// MarkNoShow move a sessão para "falta".
func (s *SessionService) MarkNoShow(ctx context.Context, id string) (*domain.Session, error) {
	return s.simpleTransition(ctx, id, domain.SessionFalta)
}

// FinishInput são os flags explícitos da finalização.
type FinishInput struct {
	Bill         bool
	ConsiderCost bool
}

// Finish move a sessão para "realizada", grava os flags e dispara os hooks
// (financeiro/custos). Os flags são decididos explicitamente pelo terapeuta.
func (s *SessionService) Finish(ctx context.Context, id string, in FinishInput) (*domain.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sess.CanTransition(domain.SessionRealizada) {
		return nil, NewValidation("A sessão não pode ser finalizada a partir do estado atual.")
	}
	sess.Status = domain.SessionRealizada
	sess.Bill = in.Bill
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
