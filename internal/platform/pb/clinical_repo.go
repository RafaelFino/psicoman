package pb

import (
	"context"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
)

// prontuarioRepo implementa app.ProntuarioRepository sobre as coleções anamnesis
// e note. É o ÚNICO repositório com cifra por campo na borda (D6.3): o conteúdo
// clínico (SealedText) é cifrado ao gravar (domain.SealFor) e decifrado ao ler
// (domain.OpenInto). O banco guarda SOMENTE o envelope; o claro nunca é escrito
// (jamais rec.Set("conteudo", agg.Content.Reveal())).
type prontuarioRepo struct {
	app    core.App
	cipher domain.Cipher
}

var _ app.ProntuarioRepository = (*prontuarioRepo)(nil)

// NewProntuarioRepository cria o repositório de prontuário com a cifra na borda.
func NewProntuarioRepository(a core.App, cipher domain.Cipher) app.ProntuarioRepository {
	return &prontuarioRepo{app: a, cipher: cipher}
}

// GetAnamnesis devolve a anamnese do paciente, decifrando o conteúdo clínico.
func (r *prontuarioRepo) GetAnamnesis(ctx context.Context, patientID string) (*clinical.Anamnesis, error) {
	rec, err := r.app.FindFirstRecordByFilter("anamnesis", "patient_id = {:p}", dbxParams("p", patientID))
	if err != nil {
		return nil, mapError(err)
	}
	content, err := domain.OpenInto(ctx, r.cipher, rec.GetString("conteudo"))
	if err != nil {
		return nil, err
	}
	return &clinical.Anamnesis{
		ID:        rec.Id,
		PatientID: rec.GetString("patient_id"),
		Content:   content,
		CreatedAt: parseTime(rec.GetString("created_at")),
		UpdatedAt: parseTime(rec.GetString("updated_at")),
	}, nil
}

// UpsertAnamnesis grava a anamnese (1:1 com o paciente, UNIQUE patient_id),
// cifrando o conteúdo clínico na borda. Atualiza a existente ou cria uma nova.
func (r *prontuarioRepo) UpsertAnamnesis(ctx context.Context, a *clinical.Anamnesis) error {
	rec, err := r.app.FindFirstRecordByFilter("anamnesis", "patient_id = {:p}", dbxParams("p", a.PatientID))
	if err != nil && !isNoRows(err) {
		return mapError(err)
	}
	if rec == nil {
		col, cerr := r.app.FindCollectionByNameOrId("anamnesis")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
		rec.Set("id", a.ID)
		rec.Set("patient_id", a.PatientID)
		rec.Set("created_at", formatTime(a.CreatedAt))
	}
	envelope, err := domain.SealFor(ctx, r.cipher, a.Content)
	if err != nil {
		return err
	}
	rec.Set("conteudo", envelope)
	rec.Set("updated_at", formatTime(a.UpdatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// CreateNote grava uma nota de prontuário, cifrando o conteúdo clínico na borda.
func (r *prontuarioRepo) CreateNote(ctx context.Context, n *clinical.Note) error {
	col, err := r.app.FindCollectionByNameOrId("note")
	if err != nil {
		return mapError(err)
	}
	envelope, err := domain.SealFor(ctx, r.cipher, n.Content)
	if err != nil {
		return err
	}
	rec := core.NewRecord(col)
	rec.Set("id", n.ID)
	rec.Set("patient_id", n.PatientID)
	rec.Set("session_id", n.SessionID)
	rec.Set("conteudo", envelope)
	rec.Set("created_at", formatTime(n.CreatedAt))
	rec.Set("updated_at", formatTime(n.UpdatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// ListNotes devolve as notas do paciente (ordenadas por created_at), decifrando
// cada conteúdo clínico na borda.
func (r *prontuarioRepo) ListNotes(ctx context.Context, patientID string) ([]*clinical.Note, error) {
	recs, err := r.app.FindRecordsByFilter("note", "patient_id = {:p}", "created_at", 0, 0, dbxParams("p", patientID))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*clinical.Note, 0, len(recs))
	for _, rec := range recs {
		content, derr := domain.OpenInto(ctx, r.cipher, rec.GetString("conteudo"))
		if derr != nil {
			return nil, derr
		}
		out = append(out, &clinical.Note{
			ID:        rec.Id,
			PatientID: rec.GetString("patient_id"),
			SessionID: rec.GetString("session_id"),
			Content:   content,
			CreatedAt: parseTime(rec.GetString("created_at")),
			UpdatedAt: parseTime(rec.GetString("updated_at")),
		})
	}
	return out, nil
}

// DeleteNote remove uma nota (hard delete: notas não têm soft-delete no MER).
func (r *prontuarioRepo) DeleteNote(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "note", id)
	if err != nil {
		return err
	}
	return mapError(r.app.DeleteWithContext(ctx, rec))
}
