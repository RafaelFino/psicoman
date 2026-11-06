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
	audit *AuditService // opcional; nil = sem auditoria (best-effort)
}

// NewProntuarioService cria o serviço de prontuário.
func NewProntuarioService(repo ProntuarioRepository, clk clock.Clock) *ProntuarioService {
	if clk == nil {
		clk = clock.System{}
	}
	return &ProntuarioService{repo: repo, clock: clk}
}

// SetAudit injeta o serviço de auditoria (mesmo padrão de SessionService). O
// ator vem do context (domain.ActorFrom via AuditService.Record), então as
// assinaturas de GetChart/AddNote/UpsertAnamnesis não mudam.
func (s *ProntuarioService) SetAudit(a *AuditService) { s.audit = a }

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
	// Auditoria best-effort, só após sucesso. meta sem texto clínico.
	if s.audit != nil {
		_ = s.audit.Record(ctx, "", domain.AuditActionCreate, "note", n.ID,
			map[string]any{"patient_id": n.PatientID, "session_id": n.SessionID})
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
	created := false
	existing, err := s.repo.GetAnamnesis(ctx, in.PatientID)
	switch {
	case err == nil && existing != nil:
		a.ID = existing.ID
		a.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
		a.ID = ulid.New()
		a.CreatedAt = now
		created = true
	default:
		return nil, err
	}
	if err := a.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.UpsertAnamnesis(ctx, a); err != nil {
		return nil, err
	}
	// Auditoria best-effort, só após sucesso. Criar vs atualizar; meta sem claro.
	if s.audit != nil {
		action := domain.AuditActionUpdate
		if created {
			action = domain.AuditActionCreate
		}
		_ = s.audit.Record(ctx, "", action, "anamnesis", a.ID,
			map[string]any{"patient_id": a.PatientID})
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
	// Auditoria da leitura clínica (R1), best-effort, só após sucesso. meta
	// carrega só a forma do prontuário (contagem de notas e presença de
	// anamnese), nunca o texto.
	if s.audit != nil {
		_ = s.audit.Record(ctx, "", domain.AuditActionRead, "prontuario", patientID,
			map[string]any{"patient_id": patientID, "notas": len(notes), "anamnese": anam != nil})
	}
	return anam, notes, nil
}
