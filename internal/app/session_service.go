package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// noopConflictChecker nunca acusa conflito (usado antes da integração Google).
type noopConflictChecker struct{}

func (noopConflictChecker) HasConflict(context.Context, time.Time, time.Time) (bool, error) {
	return false, nil
}

// SessionService orquestra o ciclo de vida das sessões. Enxuto: as transições
// que mutam a sessão e preenchem metadados vivem no agregado scheduling.Session;
// o serviço coordena I/O (persistência), checagem de conflito, criação de evento
// no Calendar, auditoria e disparo dos hooks de finalização.
// Portado de c3ab585:internal/service/session.go.
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

// NewSessionService cria o serviço. Sem Google, usa o checker no-op.
func NewSessionService(repo SessionRepository, patients PatientRepository) *SessionService {
	return &SessionService{
		repo:     repo,
		patients: patients,
		conflict: noopConflictChecker{},
		clock:    clock.System{},
	}
}

// SetAudit injeta o serviço de auditoria para registrar operações sensíveis de
// sessão (confirmação, cancelamento, remarcação). Best-effort.
func (s *SessionService) SetAudit(a *AuditService) { s.audit = a }

// SetConflictChecker injeta o verificador de conflito.
func (s *SessionService) SetConflictChecker(c ConflictChecker) {
	if c != nil {
		s.conflict = c
	}
}

// SetCalendar injeta o cliente de calendário e os reminders configurados. Ao
// agendar, cria evento + Meet + convidado.
func (s *SessionService) SetCalendar(cal SessionCalendar, reminderMinutes []int) {
	s.calendar = cal
	s.reminders = reminderMinutes
}

// AddFinishedHook registra um hook de finalização (financeiro/custos).
func (s *SessionService) AddFinishedHook(h SessionFinishedHook) {
	s.hooks = append(s.hooks, h)
}

// SessionCreateInput são os dados de criação de uma sessão.
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
// conflito de agenda e cria o evento de Calendar.
func (s *SessionService) Create(ctx context.Context, in SessionCreateInput) (*scheduling.Session, error) {
	if _, err := s.patients.Get(ctx, in.PatientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	status := in.Status
	if status == "" {
		status = scheduling.SessionAgendada
	}
	sess := &scheduling.Session{
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
	if status == scheduling.SessionAgendada {
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

// Update remarca uma sessão (data/hora, local, modalidade) via o agregado, sem
// trocar o paciente. Checa conflito quando a sessão está agendada e grava o
// audit session.reschedule quando o horário efetivamente muda (R4.3).
func (s *SessionService) Update(ctx context.Context, id string, in SessionUpdateInput) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	fromStart := sess.StartsAt
	changed, err := sess.Reschedule(in.StartsAt, in.EndsAt, in.LocationID, in.Modality, s.clock.Now())
	if err != nil {
		return nil, NewValidation(err.Error())
	}
	if sess.Status == scheduling.SessionAgendada {
		if err := s.ensureNoConflict(ctx, sess); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	if changed && s.audit != nil &&
		(sess.Status == scheduling.SessionAgendada || sess.Status == scheduling.SessionConfirmada) {
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
// checar conflito de agenda e criar o evento de Calendar.
func (s *SessionService) Schedule(ctx context.Context, id string) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoConflict(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.createCalendarEvent(ctx, sess); err != nil {
		return nil, err
	}
	if err := sess.Schedule(s.clock.Now()); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// Confirm move a sessão de "agendada" para "confirmada" via o agregado e audita
// session.confirm (sem conteúdo clínico).
func (s *SessionService) Confirm(ctx context.Context, id string) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := sess.Confirm(s.clock.Now()); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionSessionConfirm, "session", sess.ID,
			map[string]any{"confirmed_at": clock.Format(sess.ConfirmedAt)})
	}
	return sess, nil
}

// Cancel move a sessão para "cancelada" via o agregado (que valida o autor) e
// audita session.cancel com metadata {canceled_by} — o motivo NÃO entra em
// metadata (coerente com §4.1).
func (s *SessionService) Cancel(ctx context.Context, id, by, reason string) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := sess.Cancel(by, reason, s.clock.Now()); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionSessionCancel, "session", sess.ID,
			map[string]any{"canceled_by": by})
	}
	return sess, nil
}

// MarkNoShow move a sessão para "falta" via o agregado.
func (s *SessionService) MarkNoShow(ctx context.Context, id string) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := sess.MarkNoShow(s.clock.Now()); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// FinishInput são os flags explícitos da finalização. Bill é opcional (ponteiro)
// para distinguir "não informado" (padrão = cobrar) de opt-out explícito
// (false) — ver patient-timeline R1.3.
type FinishInput struct {
	Bill         *bool
	ConsiderCost bool
}

// Finish move a sessão para "realizada" via o agregado (que grava os flags) e
// dispara os hooks (financeiro/custos).
func (s *SessionService) Finish(ctx context.Context, id string, in FinishInput) (*scheduling.Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := sess.Finish(in.Bill, in.ConsiderCost, s.clock.Now()); err != nil {
		return nil, NewValidation(err.Error())
	}
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
func (s *SessionService) Get(ctx context.Context, id string) (*scheduling.Session, error) {
	return s.repo.Get(ctx, id)
}

// List devolve todas as sessões.
func (s *SessionService) List(ctx context.Context) ([]*scheduling.Session, error) {
	return s.repo.List(ctx)
}

// ListByPatient devolve as sessões de um paciente.
func (s *SessionService) ListByPatient(ctx context.Context, patientID string) ([]*scheduling.Session, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// ListByRange devolve as sessões cujo início está no intervalo [from, to).
func (s *SessionService) ListByRange(ctx context.Context, from, to time.Time) ([]*scheduling.Session, error) {
	return s.repo.ListByRange(ctx, from, to)
}

// createCalendarEvent cria o evento no Calendar (com Meet + convidado +
// reminders) e grava o event_id/meet_url na sessão. No-op se sem Calendar.
func (s *SessionService) createCalendarEvent(ctx context.Context, sess *scheduling.Session) error {
	if s.calendar == nil {
		return nil
	}
	p, err := s.patients.Get(ctx, sess.PatientID)
	if err != nil {
		return err
	}
	res, err := s.calendar.CreateEvent(ctx, CalendarEvent{
		Summary:         "Sessão — " + p.Name,
		Description:     "Atendimento psicológico.",
		StartsAt:        sess.StartsAt,
		EndsAt:          sess.EndsAt,
		AttendeeEmail:   p.Email,
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

func (s *SessionService) ensureNoConflict(ctx context.Context, sess *scheduling.Session) error {
	conflict, err := s.conflict.HasConflict(ctx, sess.StartsAt, sess.EndsAt)
	if err != nil {
		return err
	}
	if conflict {
		return NewConflict("Já existe um compromisso na agenda nesse horário.")
	}
	return nil
}
