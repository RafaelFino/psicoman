package app

import (
	"context"
	"errors"

	"github.com/RafaelFino/psicoman/internal/domain/therapist"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// TherapistService orquestra o perfil do terapeuta (R3). Há exatamente um perfil
// por instância (singleton lógico): Upsert reusa o id do perfil existente, de
// modo que gravar duas vezes não cria um segundo registro (idempotente 1:1). A
// validação de shape vive no agregado therapist; o serviço coordena I/O.
type TherapistService struct {
	repo  TherapistRepository
	clock clock.Clock
}

// NewTherapistService cria o serviço de perfil do terapeuta.
func NewTherapistService(repo TherapistRepository) *TherapistService {
	return &TherapistService{repo: repo, clock: clock.System{}}
}

// Get devolve o perfil da instância com seus links. ErrNotFound quando o perfil
// ainda não foi configurado.
func (s *TherapistService) Get(ctx context.Context) (*therapist.TherapistProfile, []*therapist.TherapistPlatformLink, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, nil, err
	}
	links, err := s.repo.ListLinks(ctx, p.ID)
	if err != nil {
		return nil, nil, err
	}
	return p, links, nil
}

// UpsertProfileInput são os campos editáveis do perfil do terapeuta.
type UpsertProfileInput struct {
	Name        string
	CRP         string
	Email       string
	Bio         string
	Contacts    map[string]string
	PhotoFileID string
	LocationIDs []string
}

// Upsert cria ou atualiza o perfil singleton. Se já existe um perfil, reusa o
// seu id (e preserva created_at), garantindo 1:1 por instância — gravar de novo
// atualiza o mesmo registro, nunca duplica.
func (s *TherapistService) Upsert(ctx context.Context, in UpsertProfileInput) (*therapist.TherapistProfile, error) {
	now := s.clock.Now()
	p := &therapist.TherapistProfile{
		Name:        in.Name,
		CRP:         in.CRP,
		Email:       in.Email,
		Bio:         in.Bio,
		Contacts:    in.Contacts,
		PhotoFileID: in.PhotoFileID,
		LocationIDs: in.LocationIDs,
		UpdatedAt:   now,
	}

	existing, err := s.repo.GetProfile(ctx)
	switch {
	case err == nil:
		// Atualiza o perfil existente: reusa id e preserva created_at (1:1).
		p.ID = existing.ID
		p.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
		// Primeiro perfil da instância.
		p.ID = ulid.New()
		p.CreatedAt = now
	default:
		return nil, err
	}

	if err := p.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.SaveProfile(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// AddLinkInput são os dados de um novo link de plataforma.
type AddLinkInput struct {
	Label    string
	URL      string
	OriginID string
}

// AddLink cria um link de plataforma vinculado ao perfil da instância. Falha com
// ErrNotFound se o perfil ainda não existe (o link precisa de um perfil-pai).
func (s *TherapistService) AddLink(ctx context.Context, in AddLinkInput) (*therapist.TherapistPlatformLink, error) {
	p, err := s.repo.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	l := &therapist.TherapistPlatformLink{
		ID:        ulid.New(),
		ProfileID: p.ID,
		Label:     in.Label,
		URL:       in.URL,
		OriginID:  in.OriginID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateLink(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// RemoveLink remove um link de plataforma por id.
func (s *TherapistService) RemoveLink(ctx context.Context, id string) error {
	return s.repo.DeleteLink(ctx, id)
}
