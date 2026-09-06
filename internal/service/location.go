package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// LocationRepository persiste locais e suas disponibilidades.
type LocationRepository interface {
	Create(ctx context.Context, l *domain.Location) error
	Update(ctx context.Context, l *domain.Location) error
	Get(ctx context.Context, id string) (*domain.Location, error)
	List(ctx context.Context) ([]*domain.Location, error)
	SoftDelete(ctx context.Context, id string) error

	AddAvailability(ctx context.Context, a *domain.Availability) error
	ListAvailability(ctx context.Context, locationID string) ([]*domain.Availability, error)
	DeleteAvailability(ctx context.Context, id string) error
}

// LocationService orquestra o cadastro de locais e disponibilidades.
type LocationService struct {
	repo  LocationRepository
	clock clock.Clock
}

// NewLocationService cria o serviço.
func NewLocationService(repo LocationRepository) *LocationService {
	return &LocationService{repo: repo, clock: clock.System{}}
}

// LocationInput são os dados de um local.
type LocationInput struct {
	Name       string
	Address    string
	Modality   string
	CostAmount int64
	CostPeriod string
}

// Create valida e cria um local.
func (s *LocationService) Create(ctx context.Context, in LocationInput) (*domain.Location, error) {
	now := s.clock.Now()
	l := &domain.Location{
		ID:         ulid.New(),
		Name:       in.Name,
		Address:    in.Address,
		Modality:   in.Modality,
		CostAmount: in.CostAmount,
		CostPeriod: in.CostPeriod,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if l.CostPeriod == "" {
		l.CostPeriod = domain.PeriodPorSessao
	}
	if err := l.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Update valida e atualiza um local.
func (s *LocationService) Update(ctx context.Context, id string, in LocationInput) (*domain.Location, error) {
	l, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	l.Name = in.Name
	l.Address = in.Address
	l.Modality = in.Modality
	l.CostAmount = in.CostAmount
	l.CostPeriod = in.CostPeriod
	l.UpdatedAt = s.clock.Now()
	if err := l.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Update(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Get devolve um local por id.
func (s *LocationService) Get(ctx context.Context, id string) (*domain.Location, error) {
	return s.repo.Get(ctx, id)
}

// List devolve os locais ativos.
func (s *LocationService) List(ctx context.Context) ([]*domain.Location, error) {
	return s.repo.List(ctx)
}

// Delete faz soft-delete de um local.
func (s *LocationService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

// AvailabilityInput são os dados de uma janela de disponibilidade.
type AvailabilityInput struct {
	Weekday   int
	StartTime string
	EndTime   string
	Capacity  int
}

// AddAvailability adiciona uma janela de disponibilidade a um local.
func (s *LocationService) AddAvailability(ctx context.Context, locationID string, in AvailabilityInput) (*domain.Availability, error) {
	if _, err := s.repo.Get(ctx, locationID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	a := &domain.Availability{
		ID:         ulid.New(),
		LocationID: locationID,
		Weekday:    in.Weekday,
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
		Capacity:   in.Capacity,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if a.Capacity == 0 {
		a.Capacity = 1
	}
	if err := a.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.AddAvailability(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListAvailability devolve as janelas de disponibilidade de um local.
func (s *LocationService) ListAvailability(ctx context.Context, locationID string) ([]*domain.Availability, error) {
	return s.repo.ListAvailability(ctx, locationID)
}

// DeleteAvailability remove uma janela de disponibilidade.
func (s *LocationService) DeleteAvailability(ctx context.Context, id string) error {
	return s.repo.DeleteAvailability(ctx, id)
}
