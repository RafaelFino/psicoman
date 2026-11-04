package app

import (
	"context"
	"errors"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// AppointmentService orquestra os pedidos de agendamento do paciente (R1.8): o
// paciente aprovado cria um pedido (registro interno, sem tocar o Google); o
// terapeuta confirma (cria a sessão agendada, com checagem de conflito e evento
// de Calendar via SessionService) ou recusa. Só pedidos pendentes mudam de estado.
type AppointmentService struct {
	repo      AppointmentRepository
	patients  PatientRepository
	locations LocationRepository
	sessions  *SessionService
	clock     clock.Clock
}

// NewAppointmentService cria o serviço de pedidos de agendamento.
func NewAppointmentService(repo AppointmentRepository, patients PatientRepository, locations LocationRepository, sessions *SessionService) *AppointmentService {
	return &AppointmentService{repo: repo, patients: patients, locations: locations, sessions: sessions, clock: clock.System{}}
}

// AppointmentInput são os dados de um pedido de agendamento.
type AppointmentInput struct {
	LocationID string
	SlotStart  time.Time
	SlotEnd    time.Time
	Note       string
}

// Request cria um pedido pendente para o paciente. Exige paciente aprovado
// (defesa em profundidade: o portal já barra com 403 e o hook de modelo também)
// e, se informado, um local existente.
func (s *AppointmentService) Request(ctx context.Context, patientID string, in AppointmentInput) (*scheduling.AppointmentRequest, error) {
	p, err := s.patients.Get(ctx, patientID)
	if err != nil {
		return nil, err
	}
	if p.ApprovalStatus != patient.PatientApproved {
		return nil, NewValidation("Agendamento exige paciente aprovado.")
	}
	if in.LocationID != "" {
		if _, err := s.locations.Get(ctx, in.LocationID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, NewValidation("O local informado não existe.")
			}
			return nil, err
		}
	}
	now := s.clock.Now()
	a := &scheduling.AppointmentRequest{
		ID:         ulid.New(),
		PatientID:  patientID,
		LocationID: in.LocationID,
		SlotStart:  in.SlotStart,
		SlotEnd:    in.SlotEnd,
		Status:     scheduling.ApptPendente,
		Note:       in.Note,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := a.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListPending devolve a fila de pedidos pendentes (para o terapeuta).
func (s *AppointmentService) ListPending(ctx context.Context) ([]*scheduling.AppointmentRequest, error) {
	return s.repo.ListPending(ctx)
}

// ListByPatient devolve os pedidos do paciente (qualquer status).
func (s *AppointmentService) ListByPatient(ctx context.Context, patientID string) ([]*scheduling.AppointmentRequest, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// Confirm confirma um pedido pendente: cria a sessão agendada (conflito de
// agenda → ErrConflict, o pedido segue pendente) e marca o pedido confirmado.
func (s *AppointmentService) Confirm(ctx context.Context, id string) (*scheduling.Session, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Status != scheduling.ApptPendente {
		return nil, NewConflict("O pedido já foi respondido.")
	}
	modality := scheduling.ModalityOnline
	if a.LocationID != "" {
		loc, lerr := s.locations.Get(ctx, a.LocationID)
		if lerr != nil {
			return nil, lerr
		}
		modality = loc.Modality
	}
	sess, err := s.sessions.Create(ctx, SessionCreateInput{
		PatientID:  a.PatientID,
		LocationID: a.LocationID,
		RequestID:  a.ID,
		Modality:   modality,
		StartsAt:   a.SlotStart,
		EndsAt:     a.SlotEnd,
		Status:     scheduling.SessionAgendada,
	})
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, a.ID, scheduling.ApptConfirmado); err != nil {
		return nil, err
	}
	return sess, nil
}

// Refuse recusa um pedido pendente.
func (s *AppointmentService) Refuse(ctx context.Context, id string) error {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.Status != scheduling.ApptPendente {
		return NewConflict("O pedido já foi respondido.")
	}
	return s.repo.UpdateStatus(ctx, a.ID, scheduling.ApptRecusado)
}
