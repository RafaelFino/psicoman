package pb

import (
	"context"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// gedMetaRepo implementa app.GEDMetaRepository sobre a coleção ged_file (apenas
// metadados; o binário vive no GED/Drive, fora desta coleção).
type gedMetaRepo struct{ app core.App }

var _ app.GEDMetaRepository = (*gedMetaRepo)(nil)

// NewGEDMetaRepository cria o repositório de metadados do GED.
func NewGEDMetaRepository(a core.App) app.GEDMetaRepository { return &gedMetaRepo{app: a} }

func gedFileFromRecord(rec *core.Record) *domain.GEDFile {
	return &domain.GEDFile{
		ID:        rec.Id,
		PatientID: rec.GetString("patient_id"),
		SessionID: rec.GetString("session_id"),
		DebtID:    rec.GetString("debt_id"),
		PaymentID: rec.GetString("payment_id"),
		RelPath:   rec.GetString("rel_path"),
		MIME:      rec.GetString("mime"),
		Size:      rec.GetInt64("size"),
		SHA256:    rec.GetString("sha256"),
		CreatedAt: parseTime(rec.GetString("created_at")),
	}
}

func (r *gedMetaRepo) Insert(ctx context.Context, f *domain.GEDFile) error {
	col, err := r.app.FindCollectionByNameOrId("ged_file")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	rec.Set("id", f.ID)
	rec.Set("patient_id", f.PatientID)
	rec.Set("session_id", f.SessionID)
	rec.Set("debt_id", f.DebtID)
	rec.Set("payment_id", f.PaymentID)
	rec.Set("rel_path", f.RelPath)
	rec.Set("mime", f.MIME)
	rec.Set("size", f.Size)
	rec.Set("sha256", f.SHA256)
	rec.Set("created_at", formatTime(f.CreatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *gedMetaRepo) Get(ctx context.Context, id string) (*domain.GEDFile, error) {
	rec, err := getRecordByID(r.app, "ged_file", id)
	if err != nil {
		return nil, err
	}
	return gedFileFromRecord(rec), nil
}

// FindByHash busca dedup dentro do escopo do paciente ("" = perfil terapeuta).
// Devolve app.ErrNotFound quando não há arquivo com aquele hash no escopo.
func (r *gedMetaRepo) FindByHash(ctx context.Context, patientID, sha string) (*domain.GEDFile, error) {
	rec, err := r.app.FindFirstRecordByFilter("ged_file",
		"patient_id = {:p} && sha256 = {:sha}",
		dbx.Params{"p": patientID, "sha": sha})
	if err != nil {
		return nil, mapError(err)
	}
	return gedFileFromRecord(rec), nil
}

func (r *gedMetaRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.GEDFile, error) {
	recs, err := r.app.FindRecordsByFilter("ged_file", "patient_id = {:p}", "-created_at", 0, 0, dbxParams("p", patientID))
	if err != nil {
		return nil, mapError(err)
	}
	return gedFilesFrom(recs), nil
}

func (r *gedMetaRepo) ListByCreatedRange(ctx context.Context, from, to time.Time) ([]*domain.GEDFile, error) {
	recs, err := r.app.FindRecordsByFilter("ged_file",
		"created_at >= {:from} && created_at < {:to}", "-created_at", 0, 0,
		dbx.Params{"from": clock.Format(from), "to": clock.Format(to)})
	if err != nil {
		return nil, mapError(err)
	}
	return gedFilesFrom(recs), nil
}

func (r *gedMetaRepo) Delete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "ged_file", id)
	if err != nil {
		return err
	}
	return mapError(r.app.DeleteWithContext(ctx, rec))
}

func gedFilesFrom(recs []*core.Record) []*domain.GEDFile {
	out := make([]*domain.GEDFile, 0, len(recs))
	for _, rec := range recs {
		out = append(out, gedFileFromRecord(rec))
	}
	return out
}

// auditRepo implementa app.AuditRepository sobre a coleção audit_log.
type auditRepo struct{ app core.App }

var _ app.AuditRepository = (*auditRepo)(nil)

// NewAuditRepository cria o repositório de auditoria.
func NewAuditRepository(a core.App) app.AuditRepository { return &auditRepo{app: a} }

func auditFromRecord(rec *core.Record) *domain.AuditLog {
	var metadata map[string]any
	// O campo metadata é JSON (types.JSONRaw em repouso); desserializa para o
	// map do domínio. Vazio/ausente -> metadata nil (log sem metadados).
	_ = rec.UnmarshalJSONField("metadata", &metadata)
	return &domain.AuditLog{
		ID:        rec.Id,
		Actor:     rec.GetString("actor"),
		Action:    rec.GetString("action"),
		Entity:    rec.GetString("entity"),
		EntityID:  rec.GetString("entity_id"),
		Metadata:  metadata,
		CreatedAt: parseTime(rec.GetString("created_at")),
	}
}

func (r *auditRepo) Insert(ctx context.Context, entry *domain.AuditLog) error {
	col, err := r.app.FindCollectionByNameOrId("audit_log")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	rec.Set("id", entry.ID)
	rec.Set("actor", entry.Actor)
	rec.Set("action", entry.Action)
	rec.Set("entity", entry.Entity)
	rec.Set("entity_id", entry.EntityID)
	rec.Set("metadata", entry.Metadata)
	rec.Set("created_at", formatTime(entry.CreatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *auditRepo) List(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	recs, err := r.app.FindRecordsByFilter("audit_log", "", "-created_at", limit, 0)
	if err != nil {
		return nil, mapError(err)
	}
	return auditEntriesFrom(recs), nil
}

// ListByEntity devolve as entradas de uma entidade específica, mais recentes
// primeiro (usa o índice idx_audit_log_entity em (entity, entity_id)).
func (r *auditRepo) ListByEntity(ctx context.Context, entity, entityID string) ([]*domain.AuditLog, error) {
	recs, err := r.app.FindRecordsByFilter("audit_log",
		"entity = {:e} && entity_id = {:id}", "-created_at", 0, 0,
		dbx.Params{"e": entity, "id": entityID})
	if err != nil {
		return nil, mapError(err)
	}
	return auditEntriesFrom(recs), nil
}

func auditEntriesFrom(recs []*core.Record) []*domain.AuditLog {
	out := make([]*domain.AuditLog, 0, len(recs))
	for _, rec := range recs {
		out = append(out, auditFromRecord(rec))
	}
	return out
}
