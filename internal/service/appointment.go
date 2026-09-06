package service

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// AppointmentRepository persiste pedidos de agendamento.
type AppointmentRepository interface {
	Create(ctx context.Context, a *domain.AppointmentRequest) error
	Get(ctx context.Context, id string) (*domain.AppointmentRequest, error)
	ListPending(ctx context.Context) ([]*domain.AppointmentRequest, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.AppointmentRequest, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

// AppointmentService orquestra os pedidos de agendamento e sua confirmação.
type AppointmentService struct {
	repo      AppointmentRepository
	patients  PatientRepository
	sessions  *SessionService
	locations LocationRepository
	clock     clock.Clock
}

// NewAppointmentService cria o serviço.
func NewAppointmentService(repo AppointmentRepository, patients PatientRepository, sessions *SessionService, locations LocationRepository) *AppointmentService {
	return &AppointmentService{repo: repo, patients: patients, sessions: sessions, locations: locations, clock: clock.System{}}
}

// RequestInput são os dados de um pedido de agendamento.
type RequestInput struct {
	PatientID  string
	LocationID string
	SlotStart  time.Time
	SlotEnd    time.Time
	Note       string
}

// Request cria um pedido pendente (registro interno; nada toca o Google).
func (s *AppointmentService) Request(ctx context.Context, in RequestInput) (*domain.AppointmentRequest, error) {
	if _, err := s.patients.Get(ctx, in.PatientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	a := &domain.AppointmentRequest{
		ID: ulid.New(), PatientID: in.PatientID, LocationID: in.LocationID,
		SlotStart: in.SlotStart, SlotEnd: in.SlotEnd, Status: domain.ApptPendente,
		Note: in.Note, CreatedAt: now, UpdatedAt: now,
	}
	if err := a.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListPending devolve os pedidos pendentes (tela de pendências do admin).
func (s *AppointmentService) ListPending(ctx context.Context) ([]*domain.AppointmentRequest, error) {
	return s.repo.ListPending(ctx)
}

// ListByPatient devolve os pedidos de um paciente.
func (s *AppointmentService) ListByPatient(ctx context.Context, patientID string) ([]*domain.AppointmentRequest, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// Confirm confirma um pedido: cria a sessão agendada (checa conflito e cria o
// evento no Calendar via SessionService) e marca o pedido como confirmado.
func (s *AppointmentService) Confirm(ctx context.Context, id, modality string) (*domain.Session, error) {
	req, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != domain.ApptPendente {
		return nil, NewValidation("Este pedido já foi tratado.")
	}
	if modality == "" {
		modality = domain.ModalityOnline
	}
	// Cria a sessão já como agendada — dispara checagem de conflito + evento.
	sess, err := s.sessions.Create(ctx, SessionCreateInput{
		PatientID:  req.PatientID,
		LocationID: req.LocationID,
		RequestID:  req.ID,
		Modality:   modality,
		StartsAt:   req.SlotStart,
		EndsAt:     req.SlotEnd,
		Status:     domain.SessionAgendada,
	})
	if err != nil {
		return nil, err // conflito de agenda propaga como ErrConflict → 409
	}
	if err := s.repo.UpdateStatus(ctx, id, domain.ApptConfirmado); err != nil {
		return nil, err
	}
	return sess, nil
}

// Reject recusa um pedido pendente.
func (s *AppointmentService) Reject(ctx context.Context, id string) error {
	req, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if req.Status != domain.ApptPendente {
		return NewValidation("Este pedido já foi tratado.")
	}
	return s.repo.UpdateStatus(ctx, id, domain.ApptRecusado)
}
