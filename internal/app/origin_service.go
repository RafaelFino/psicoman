package app

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// OriginService orquestra o CRUD de canais de aquisição (origens) — base do ROI
// por canal (R1.7). A validação vive no agregado patient.Origin.
type OriginService struct {
	repo  OriginRepository
	clock clock.Clock
}

// NewOriginService cria o serviço de origens.
func NewOriginService(repo OriginRepository) *OriginService {
	return &OriginService{repo: repo, clock: clock.System{}}
}

// Create valida e cria uma origem.
func (s *OriginService) Create(ctx context.Context, name string) (*patient.Origin, error) {
	now := s.clock.Now()
	o := &patient.Origin{ID: ulid.New(), Name: name, CreatedAt: now, UpdatedAt: now}
	if err := o.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// Update valida e atualiza o nome de uma origem.
func (s *OriginService) Update(ctx context.Context, id, name string) (*patient.Origin, error) {
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Name = name
	o.UpdatedAt = s.clock.Now()
	if err := o.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// List devolve as origens ativas.
func (s *OriginService) List(ctx context.Context) ([]*patient.Origin, error) {
	return s.repo.List(ctx)
}

// Delete faz o soft-delete de uma origem.
func (s *OriginService) Delete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}
