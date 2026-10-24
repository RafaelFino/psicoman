package pb

import (
	"context"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// aiSummaryRepo implementa app.AISummaryRepository sobre a coleção ai_summary.
// O campo resumo é CLÍNICO: cifrado na borda (envelope), decifrado ao ler — como
// o prontuarioRepo (D6.3). O banco guarda só o envelope.
type aiSummaryRepo struct {
	app    core.App
	cipher domain.Cipher
}

var _ app.AISummaryRepository = (*aiSummaryRepo)(nil)

// NewAISummaryRepository cria o repositório de resumos de IA com cifra na borda.
func NewAISummaryRepository(a core.App, cipher domain.Cipher) app.AISummaryRepository {
	return &aiSummaryRepo{app: a, cipher: cipher}
}

func (r *aiSummaryRepo) Create(ctx context.Context, s *clinical.AISummary) error {
	col, err := r.app.FindCollectionByNameOrId("ai_summary")
	if err != nil {
		return mapError(err)
	}
	envelope, err := domain.SealFor(ctx, r.cipher, s.Resumo)
	if err != nil {
		return err
	}
	rec := core.NewRecord(col)
	rec.Set("id", s.ID)
	rec.Set("session_id", s.SessionID)
	rec.Set("resumo", envelope)
	rec.Set("status", s.Status)
	rec.Set("generated_at", formatTime(s.GeneratedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *aiSummaryRepo) Get(ctx context.Context, id string) (*clinical.AISummary, error) {
	rec, err := getRecordByID(r.app, "ai_summary", id)
	if err != nil {
		return nil, err
	}
	return r.fromRecord(ctx, rec)
}

func (r *aiSummaryRepo) GetBySession(ctx context.Context, sessionID string) (*clinical.AISummary, error) {
	rec, err := r.app.FindFirstRecordByFilter("ai_summary", "session_id = {:s}", dbxParams("s", sessionID))
	if err != nil {
		return nil, mapError(err)
	}
	return r.fromRecord(ctx, rec)
}

func (r *aiSummaryRepo) UpdateStatus(ctx context.Context, id, status string) error {
	rec, err := getRecordByID(r.app, "ai_summary", id)
	if err != nil {
		return err
	}
	rec.Set("status", status)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// fromRecord decifra o resumo (SealedText em memória) e monta o agregado.
func (r *aiSummaryRepo) fromRecord(ctx context.Context, rec *core.Record) (*clinical.AISummary, error) {
	resumo, err := domain.OpenInto(ctx, r.cipher, rec.GetString("resumo"))
	if err != nil {
		return nil, err
	}
	return &clinical.AISummary{
		ID:          rec.Id,
		SessionID:   rec.GetString("session_id"),
		Resumo:      resumo,
		Status:      rec.GetString("status"),
		GeneratedAt: parseTime(rec.GetString("generated_at")),
	}, nil
}

// tagIndexRepo implementa app.TagIndexRepository sobre a coleção tag_index. As
// tags ficam em CLARO (índice invertido para busca); session_ids é um JSON.
// Isolado por paciente: todo filtro inclui patient_id (R10.2/R6).
type tagIndexRepo struct{ app core.App }

var _ app.TagIndexRepository = (*tagIndexRepo)(nil)

// NewTagIndexRepository cria o repositório do índice invertido de tags.
func NewTagIndexRepository(a core.App) app.TagIndexRepository { return &tagIndexRepo{app: a} }

// Upsert adiciona sessionID à entrada (patient_id, ano, tag), criando-a se não
// existir. Merge idempotente: não duplica a sessão na lista.
func (r *tagIndexRepo) Upsert(ctx context.Context, patientID string, ano int, tag, sessionID string) error {
	rec, err := r.app.FindFirstRecordByFilter("tag_index",
		"patient_id = {:p} && ano = {:a} && tag = {:t}",
		dbx.Params{"p": patientID, "a": ano, "t": tag})
	if err != nil && !isNoRows(err) {
		return mapError(err)
	}
	if rec == nil {
		col, cerr := r.app.FindCollectionByNameOrId("tag_index")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
		rec.Set("id", ulid.New())
		rec.Set("patient_id", patientID)
		rec.Set("ano", ano)
		rec.Set("tag", tag)
		rec.Set("session_ids", []string{sessionID})
		return mapError(r.app.SaveWithContext(ctx, rec))
	}
	// Merge: adiciona o sessionID se ainda não estiver na lista.
	var ids []string
	_ = rec.UnmarshalJSONField("session_ids", &ids)
	for _, id := range ids {
		if id == sessionID {
			return nil // já indexado: idempotente
		}
	}
	ids = append(ids, sessionID)
	rec.Set("session_ids", ids)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// FindByTag devolve as sessões que carregam a tag no ano, para o paciente.
// ErrNotFound quando não há entrada para a (patient_id, ano, tag).
func (r *tagIndexRepo) FindByTag(ctx context.Context, patientID string, ano int, tag string) ([]string, error) {
	rec, err := r.app.FindFirstRecordByFilter("tag_index",
		"patient_id = {:p} && ano = {:a} && tag = {:t}",
		dbx.Params{"p": patientID, "a": ano, "t": tag})
	if err != nil {
		return nil, mapError(err)
	}
	var ids []string
	_ = rec.UnmarshalJSONField("session_ids", &ids)
	return ids, nil
}

// ListTags devolve as tags do paciente no ano (para navegação), isolado por paciente.
func (r *tagIndexRepo) ListTags(ctx context.Context, patientID string, ano int) ([]*clinical.TagIndex, error) {
	recs, err := r.app.FindRecordsByFilter("tag_index",
		"patient_id = {:p} && ano = {:a}", "tag", 0, 0,
		dbx.Params{"p": patientID, "a": ano})
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*clinical.TagIndex, 0, len(recs))
	for _, rec := range recs {
		var ids []string
		_ = rec.UnmarshalJSONField("session_ids", &ids)
		out = append(out, &clinical.TagIndex{
			ID:         rec.Id,
			PatientID:  rec.GetString("patient_id"),
			Ano:        rec.GetInt("ano"),
			Tag:        rec.GetString("tag"),
			SessionIDs: ids,
		})
	}
	return out, nil
}
