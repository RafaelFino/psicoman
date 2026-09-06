package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// OriginRepository persiste origens (canais de aquisição).
type OriginRepository interface {
	Create(ctx context.Context, o *domain.Origin) error
	Update(ctx context.Context, o *domain.Origin) error
	Get(ctx context.Context, id string) (*domain.Origin, error)
	List(ctx context.Context) ([]*domain.Origin, error)
	SoftDelete(ctx context.Context, id string) error
}

// OriginService orquestra o cadastro de origens.
type OriginService struct {
	repo  OriginRepository
	clock clock.Clock
}

// NewOriginService cria o serviço.
func NewOriginService(repo OriginRepository) *OriginService {
	return &OriginService{repo: repo, clock: clock.System{}}
}

// Create valida e cria uma origem.
func (s *OriginService) Create(ctx context.Context, name string) (*domain.Origin, error) {
	now := s.clock.Now()
	o := &domain.Origin{ID: ulid.New(), Name: name, CreatedAt: now, UpdatedAt: now}
	if err := o.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// Update atualiza o nome de uma origem.
func (s *OriginService) Update(ctx context.Context, id, name string) (*domain.Origin, error) {
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

// Get devolve uma origem por id.
func (s *OriginService) Get(ctx context.Context, id string) (*domain.Origin, error) {
	return s.repo.Get(ctx, id)
}

// List devolve as origens ativas.
func (s *OriginService) List(ctx context.Context) ([]*domain.Origin, error) {
	return s.repo.List(ctx)
}

// Delete faz soft-delete de uma origem.
func (s *OriginService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}
