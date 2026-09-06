package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/markdown"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// ProntuarioRepository persiste anamnese e notas.
type ProntuarioRepository interface {
	GetAnamnesis(ctx context.Context, patientID string) (*domain.Anamnesis, error)
	UpsertAnamnesis(ctx context.Context, a *domain.Anamnesis) error

	CreateNote(ctx context.Context, n *domain.Note) error
	ListNotes(ctx context.Context, patientID string) ([]*domain.Note, error)
	DeleteNote(ctx context.Context, id string) error
}

// TemplateRepository persiste templates e envios.
type TemplateRepository interface {
	Create(ctx context.Context, t *domain.Template) error
	Get(ctx context.Context, id string) (*domain.Template, error)
	List(ctx context.Context) ([]*domain.Template, error)
	SoftDelete(ctx context.Context, id string) error
	RecordSend(ctx context.Context, ts *domain.TemplateSend) error
}

// TemplateSender envia o HTML renderizado ao paciente (Gmail na Task 16).
// Best-effort: falha de envio não bloqueia o registro.
type TemplateSender interface {
	Send(ctx context.Context, toEmail, subject, html string) error
}

// ProntuarioService orquestra prontuário e templates.
type ProntuarioService struct {
	repo      ProntuarioRepository
	templates TemplateRepository
	patients  PatientRepository
	sender    TemplateSender
	clock     clock.Clock
}

// NewProntuarioService cria o serviço.
func NewProntuarioService(repo ProntuarioRepository, templates TemplateRepository, patients PatientRepository) *ProntuarioService {
	return &ProntuarioService{repo: repo, templates: templates, patients: patients, clock: clock.System{}}
}

// SetSender injeta o enviador de templates (Task 16).
func (s *ProntuarioService) SetSender(sender TemplateSender) { s.sender = sender }

// GetAnamnesis devolve a anamnese do paciente (vazia se ainda não existe).
func (s *ProntuarioService) GetAnamnesis(ctx context.Context, patientID string) (*domain.Anamnesis, error) {
	a, err := s.repo.GetAnamnesis(ctx, patientID)
	if err == ErrNotFound {
		return &domain.Anamnesis{PatientID: patientID}, nil
	}
	return a, err
}

// SaveAnamnesis cria ou atualiza a anamnese (uma por paciente).
func (s *ProntuarioService) SaveAnamnesis(ctx context.Context, patientID, content string) (*domain.Anamnesis, error) {
	if _, err := s.patients.Get(ctx, patientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	a, err := s.repo.GetAnamnesis(ctx, patientID)
	if err == ErrNotFound {
		a = &domain.Anamnesis{ID: ulid.New(), PatientID: patientID, CreatedAt: now}
	} else if err != nil {
		return nil, err
	}
	a.Content = content
	a.UpdatedAt = now
	if err := s.repo.UpsertAnamnesis(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// AddNote cria uma nota (de sessão se sessionID != "", livre caso contrário).
func (s *ProntuarioService) AddNote(ctx context.Context, patientID, sessionID, content string) (*domain.Note, error) {
	if _, err := s.patients.Get(ctx, patientID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	n := &domain.Note{
		ID: ulid.New(), PatientID: patientID, SessionID: sessionID,
		Content: content, CreatedAt: now, UpdatedAt: now,
	}
	if err := n.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateNote(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

// ListNotes devolve as notas do paciente ordenadas por created_at.
func (s *ProntuarioService) ListNotes(ctx context.Context, patientID string) ([]*domain.Note, error) {
	return s.repo.ListNotes(ctx, patientID)
}

// DeleteNote remove uma nota.
func (s *ProntuarioService) DeleteNote(ctx context.Context, id string) error {
	return s.repo.DeleteNote(ctx, id)
}

// CreateTemplate cria um template Markdown.
func (s *ProntuarioService) CreateTemplate(ctx context.Context, name, bodyMD string) (*domain.Template, error) {
	now := s.clock.Now()
	t := &domain.Template{ID: ulid.New(), Name: name, BodyMD: bodyMD, CreatedAt: now, UpdatedAt: now}
	if err := t.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.templates.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ListTemplates devolve os templates.
func (s *ProntuarioService) ListTemplates(ctx context.Context) ([]*domain.Template, error) {
	return s.templates.List(ctx)
}

// GetTemplate devolve um template por id.
func (s *ProntuarioService) GetTemplate(ctx context.Context, id string) (*domain.Template, error) {
	return s.templates.Get(ctx, id)
}

// DeleteTemplate remove um template.
func (s *ProntuarioService) DeleteTemplate(ctx context.Context, id string) error {
	return s.templates.SoftDelete(ctx, id)
}

// RenderTemplate devolve o HTML formatado de um template (sempre a versão
// formatada é a que se envia — requirements §3.6).
func (s *ProntuarioService) RenderTemplate(ctx context.Context, id string) (string, error) {
	t, err := s.templates.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return markdown.ToHTML(t.BodyMD), nil
}

// SendTemplate renderiza o template em HTML, registra o envio e (se houver
// sender configurado) o envia por email — best-effort.
func (s *ProntuarioService) SendTemplate(ctx context.Context, templateID, patientID string) (*domain.TemplateSend, error) {
	t, err := s.templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	patient, err := s.patients.Get(ctx, patientID)
	if err != nil {
		return nil, err
	}

	rendered := markdown.ToHTML(t.BodyMD)
	ts := &domain.TemplateSend{
		ID: ulid.New(), TemplateID: templateID, PatientID: patientID,
		RenderedHTML: rendered, SentAt: s.clock.Now(),
	}
	if err := s.templates.RecordSend(ctx, ts); err != nil {
		return nil, err
	}
	// Envio por email é best-effort (não bloqueia o registro).
	if s.sender != nil {
		_ = s.sender.Send(ctx, patient.Email, t.Name, rendered)
	}
	return ts, nil
}
