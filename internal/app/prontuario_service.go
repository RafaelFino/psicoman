package app

import (
	"context"
	"errors"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// ProntuarioService orquestra a escrita e a leitura do prontuário (anamnese +
// notas). Enxuto: as invariantes de shape vivem nos agregados clinical.Note /
// clinical.Anamnesis; o serviço gera identidade/tempo, monta o SealedText com o
// claro recebido do handler e valida antes de persistir. O repositório cifra na
// borda (D6.3) — o claro nunca é gravado no banco.
//
// Novo nesta onda (Fase 4): não havia serviço que agregasse anamnese+notas
// sobre a ProntuarioRepository; o handler fino não pode gerar identidade/tempo
// nem montar SealedText (responsabilidade de serviço, espelha PatientService).
type ProntuarioService struct {
	repo  ProntuarioRepository
	clock clock.Clock
}

// NewProntuarioService cria o serviço de prontuário.
func NewProntuarioService(repo ProntuarioRepository, clk clock.Clock) *ProntuarioService {
	if clk == nil {
		clk = clock.System{}
	}
	return &ProntuarioService{repo: repo, clock: clk}
}

// AddNoteInput são os dados de uma nova nota de prontuário. Content é o CLARO
// vindo do handler; o serviço monta o SealedText.
type AddNoteInput struct {
	PatientID string
	SessionID string // opcional: "" = nota livre
	Content   string
}

// UpsertAnamnesisInput são os dados de upsert da anamnese (1:1 por paciente).
// Content é o CLARO; o serviço monta o SealedText.
type UpsertAnamnesisInput struct {
	PatientID string
	Content   string
}

// AddNote cria uma nota de prontuário: gera id/tempo, monta o SealedText com o
// claro recebido, valida e persiste (o repo cifra na borda).
func (s *ProntuarioService) AddNote(ctx context.Context, in AddNoteInput) (*clinical.Note, error) {
	now := s.clock.Now()
	n := &clinical.Note{
		ID:        ulid.New(),
		PatientID: in.PatientID,
		SessionID: in.SessionID,
		Content:   domain.NewSealedText(in.Content),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := n.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateNote(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

// UpsertAnamnesis cria ou substitui a anamnese do paciente (idempotente 1:1):
// se já existe, mantém o ID e CreatedAt e atualiza UpdatedAt+Content; se não,
// gera id e CreatedAt novos. Valida e persiste (o repo cifra na borda).
func (s *ProntuarioService) UpsertAnamnesis(ctx context.Context, in UpsertAnamnesisInput) (*clinical.Anamnesis, error) {
	now := s.clock.Now()
	a := &clinical.Anamnesis{
		PatientID: in.PatientID,
		Content:   domain.NewSealedText(in.Content),
		UpdatedAt: now,
	}
	existing, err := s.repo.GetAnamnesis(ctx, in.PatientID)
	switch {
	case err == nil && existing != nil:
		a.ID = existing.ID
		a.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
		a.ID = ulid.New()
		a.CreatedAt = now
	default:
		return nil, err
	}
	if err := a.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.UpsertAnamnesis(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// GetChart lê a anamnese e as notas do paciente. Anamnese ausente (ErrNotFound)
// é tratada como "sem anamnese" (anamnese nil, sem erro) — o handler monta o DTO
// com anamnese opcional. O repo decifra o SealedText em memória; o handler
// revela o claro ao montar o DTO do ator app.
func (s *ProntuarioService) GetChart(ctx context.Context, patientID string) (*clinical.Anamnesis, []*clinical.Note, error) {
	anam, err := s.repo.GetAnamnesis(ctx, patientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			anam = nil
		} else {
			return nil, nil, err
		}
	}
	notes, err := s.repo.ListNotes(ctx, patientID)
	if err != nil {
		return nil, nil, err
	}
	return anam, notes, nil
}
