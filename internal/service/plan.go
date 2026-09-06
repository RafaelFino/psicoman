package service

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PlanRepository persiste planos por paciente.
type PlanRepository interface {
	Create(ctx context.Context, p *domain.Plan) error
	Update(ctx context.Context, p *domain.Plan) error
	Get(ctx context.Context, id string) (*domain.Plan, error)
	// GetActiveByPatient devolve o plano vigente do paciente em ref (ErrNotFound se nenhum).
	GetActiveByPatient(ctx context.Context, patientID string, ref time.Time) (*domain.Plan, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.Plan, error)
	// ListFixedCycleActive devolve planos fechados vigentes em ref.
	ListFixedCycleActive(ctx context.Context, ref time.Time) ([]*domain.Plan, error)
	SoftDelete(ctx context.Context, id string) error
}

// PlanService orquestra os planos.
type PlanService struct {
	repo     PlanRepository
	patients PatientRepository
	clock    clock.Clock
}

// NewPlanService cria o serviço.
func NewPlanService(repo PlanRepository, patients PatientRepository) *PlanService {
	return &PlanService{repo: repo, patients: patients, clock: clock.System{}}
}

// PlanInput são os dados de um plano.
type PlanInput struct {
	PatientID string
	Type      string
	Amount    int64
	StartsAt  time.Time
	EndsAt    time.Time
}

// Create valida e cria um plano.
func (s *PlanService) Create(ctx context.Context, in PlanInput) (*domain.Plan, error) {
	if _, err := s.patients.Get(ctx, in.PatientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	starts := in.StartsAt
	if starts.IsZero() {
		starts = now
	}
	p := &domain.Plan{
		ID: ulid.New(), PatientID: in.PatientID, Type: in.Type, Amount: in.Amount,
		StartsAt: starts, EndsAt: in.EndsAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get devolve um plano por id.
func (s *PlanService) Get(ctx context.Context, id string) (*domain.Plan, error) {
	return s.repo.Get(ctx, id)
}

// ListByPatient devolve os planos de um paciente.
func (s *PlanService) ListByPatient(ctx context.Context, patientID string) ([]*domain.Plan, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// Delete faz soft-delete de um plano.
func (s *PlanService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}
