package app

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// LocationService orquestra o CRUD de locais de atendimento e suas janelas de
// disponibilidade (R1.7). Enxuto: a validação de shape (modalidade, periodicidade,
// horários) vive nos agregados scheduling.Location/Availability; o serviço
// coordena I/O e gera ULID/tempo.
type LocationService struct {
	repo  LocationRepository
	clock clock.Clock
}

// NewLocationService cria o serviço de locais.
func NewLocationService(repo LocationRepository) *LocationService {
	return &LocationService{repo: repo, clock: clock.System{}}
}

// LocationInput são os campos editáveis de um local.
type LocationInput struct {
	Name       string
	Address    string
	Modality   string
	CostAmount int64
	CostPeriod string
}

// Create valida e cria um local.
func (s *LocationService) Create(ctx context.Context, in LocationInput) (*scheduling.Location, error) {
	now := s.clock.Now()
	l := &scheduling.Location{
		ID:         ulid.New(),
		Name:       in.Name,
		Address:    in.Address,
		Modality:   in.Modality,
		CostAmount: in.CostAmount,
		CostPeriod: in.CostPeriod,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := l.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Update valida e atualiza um local existente (full-replace dos editáveis).
func (s *LocationService) Update(ctx context.Context, id string, in LocationInput) (*scheduling.Location, error) {
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
func (s *LocationService) Get(ctx context.Context, id string) (*scheduling.Location, error) {
	return s.repo.Get(ctx, id)
}

// List devolve todos os locais ativos.
func (s *LocationService) List(ctx context.Context) ([]*scheduling.Location, error) {
	return s.repo.List(ctx)
}

// Delete faz o soft-delete de um local.
func (s *LocationService) Delete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}

// AvailabilityInput são os campos de uma janela de disponibilidade.
type AvailabilityInput struct {
	Weekday   int
	StartTime string
	EndTime   string
	Capacity  int
}

// AddAvailability valida e adiciona uma janela de disponibilidade a um local.
func (s *LocationService) AddAvailability(ctx context.Context, locationID string, in AvailabilityInput) (*scheduling.Availability, error) {
	// Garante que o local existe (ErrNotFound propaga como 404).
	if _, err := s.repo.Get(ctx, locationID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	a := &scheduling.Availability{
		ID:         ulid.New(),
		LocationID: locationID,
		Weekday:    in.Weekday,
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
		Capacity:   in.Capacity,
		CreatedAt:  now,
		UpdatedAt:  now,
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
func (s *LocationService) ListAvailability(ctx context.Context, locationID string) ([]*scheduling.Availability, error) {
	return s.repo.ListAvailability(ctx, locationID)
}

// DeleteAvailability remove uma janela de disponibilidade por id.
func (s *LocationService) DeleteAvailability(ctx context.Context, id string) error {
	return s.repo.DeleteAvailability(ctx, id)
}
