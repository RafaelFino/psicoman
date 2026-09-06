package service

import (
	"context"
	"io"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// TherapistRepository persiste o perfil do terapeuta (um por instância), suas
// associações com locais e os links de plataforma.
type TherapistRepository interface {
	// GetProfile devolve o perfil (ErrNotFound se ainda não existe).
	GetProfile(ctx context.Context) (*domain.TherapistProfile, error)
	// UpsertProfile cria ou atualiza o perfil único.
	UpsertProfile(ctx context.Context, p *domain.TherapistProfile) error
	// SetLocations substitui as associações de locais.
	SetLocations(ctx context.Context, profileID string, locationIDs []string) error

	AddLink(ctx context.Context, l *domain.TherapistPlatformLink) error
	ListLinks(ctx context.Context, profileID string) ([]*domain.TherapistPlatformLink, error)
	DeleteLink(ctx context.Context, id string) error
}

// TherapistService orquestra o perfil do terapeuta.
type TherapistService struct {
	repo  TherapistRepository
	ged   *GEDService
	clock clock.Clock
}

// NewTherapistService cria o serviço.
func NewTherapistService(repo TherapistRepository, ged *GEDService) *TherapistService {
	return &TherapistService{repo: repo, ged: ged, clock: clock.System{}}
}

// ProfileInput são os dados editáveis do perfil.
type ProfileInput struct {
	Name        string
	CRP         string
	Email       string
	Contacts    map[string]string
	Bio         string
	LocationIDs []string
}

// SaveProfile cria ou atualiza o perfil único da instância.
func (s *TherapistService) SaveProfile(ctx context.Context, in ProfileInput) (*domain.TherapistProfile, error) {
	existing, err := s.repo.GetProfile(ctx)
	now := s.clock.Now()

	var p *domain.TherapistProfile
	switch {
	case err == nil:
		p = existing
	case err == ErrNotFound:
		p = &domain.TherapistProfile{ID: ulid.New(), CreatedAt: now}
	default:
		return nil, err
	}

	p.Name = in.Name
	p.CRP = in.CRP
	p.Email = in.Email
	p.Contacts = in.Contacts
	p.Bio = in.Bio
	p.LocationIDs = in.LocationIDs
	p.UpdatedAt = now

	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.UpsertProfile(ctx, p); err != nil {
		return nil, err
	}
	if err := s.repo.SetLocations(ctx, p.ID, in.LocationIDs); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProfile devolve o perfil atual (ErrNotFound se ainda não configurado).
func (s *TherapistService) GetProfile(ctx context.Context) (*domain.TherapistProfile, error) {
	return s.repo.GetProfile(ctx)
}

// SetPhoto armazena a foto no GED (escopo do terapeuta) e vincula ao perfil.
func (s *TherapistService) SetPhoto(ctx context.Context, mime string, content io.Reader) (*domain.TherapistProfile, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	f, err := s.ged.Store(ctx, GEDLink{}, mime, content) // escopo vazio = terapeuta
	if err != nil {
		return nil, err
	}
	p.PhotoFileID = f.ID
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.UpsertProfile(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// AddLink adiciona um link de plataforma ao perfil.
func (s *TherapistService) AddLink(ctx context.Context, label, url, originID string) (*domain.TherapistPlatformLink, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	l := &domain.TherapistPlatformLink{
		ID: ulid.New(), ProfileID: p.ID, Label: label, URL: url, OriginID: originID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := l.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.AddLink(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// ListLinks devolve os links de plataforma do perfil.
func (s *TherapistService) ListLinks(ctx context.Context) ([]*domain.TherapistPlatformLink, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.ListLinks(ctx, p.ID)
}

// DeleteLink remove um link de plataforma.
func (s *TherapistService) DeleteLink(ctx context.Context, id string) error {
	return s.repo.DeleteLink(ctx, id)
}
