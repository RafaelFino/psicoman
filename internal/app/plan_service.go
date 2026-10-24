package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PlanService orquestra o CRUD de planos de cobrança por paciente (R1.9). Um
// plano vigente é o que habilita a geração de débito por sessão finalizada
// (BillingService.OnSessionFinished lê GetActiveByPatient). A validação de
// shape (tipo, valor fixo, dia de vencimento, vigência) vive no agregado
// billing.Plan.
type PlanService struct {
	repo  PlanRepository
	clock clock.Clock
}

// NewPlanService cria o serviço de planos.
func NewPlanService(repo PlanRepository) *PlanService {
	return &PlanService{repo: repo, clock: clock.System{}}
}

// PlanInput são os campos de um plano. StartsAt/EndsAt em America/Sao_Paulo;
// EndsAt zero = sem fim.
type PlanInput struct {
	PatientID string
	Type      string
	Amount    int64
	DueDay    int
	StartsAt  time.Time
	EndsAt    time.Time
}

// Create valida e cria um plano para o paciente.
func (s *PlanService) Create(ctx context.Context, in PlanInput) (*billing.Plan, error) {
	now := s.clock.Now()
	p := &billing.Plan{
		ID:        ulid.New(),
		PatientID: in.PatientID,
		Type:      in.Type,
		Amount:    in.Amount,
		DueDay:    in.DueDay,
		StartsAt:  in.StartsAt,
		EndsAt:    in.EndsAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListByPatient devolve os planos de um paciente.
func (s *PlanService) ListByPatient(ctx context.Context, patientID string) ([]*billing.Plan, error) {
	return s.repo.ListByPatient(ctx, patientID)
}

// Get devolve um plano por id.
func (s *PlanService) Get(ctx context.Context, id string) (*billing.Plan, error) {
	return s.repo.Get(ctx, id)
}

// ActiveByPatient devolve o plano vigente do paciente na data ref (ErrNotFound
// se nenhum). Útil para a UI mostrar o plano corrente.
func (s *PlanService) ActiveByPatient(ctx context.Context, patientID string, ref time.Time) (*billing.Plan, error) {
	return s.repo.GetActiveByPatient(ctx, patientID, ref)
}

// Delete faz o soft-delete de um plano.
func (s *PlanService) Delete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}
