package pb

import (
	"context"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// patientRepo implementa app.PatientRepository sobre a coleção patient.
type patientRepo struct{ app core.App }

var _ app.PatientRepository = (*patientRepo)(nil)

// NewPatientRepository cria o repositório de pacientes.
func NewPatientRepository(a core.App) app.PatientRepository { return &patientRepo{app: a} }

func (r *patientRepo) toRecord(rec *core.Record, p *patient.Patient) {
	rec.Set("id", p.ID)
	rec.Set("name", p.Name)
	rec.Set("phone", p.Phone)
	rec.Set("email", p.Email)
	rec.Set("cpf", p.CPF)
	rec.Set("origin_id", p.OriginID)
	rec.Set("approval_status", p.ApprovalStatus)
	rec.Set("created_at", formatTime(p.CreatedAt))
	rec.Set("updated_at", formatTime(p.UpdatedAt))
}

func patientFromRecord(rec *core.Record) *patient.Patient {
	return &patient.Patient{
		ID:             rec.Id,
		Name:           rec.GetString("name"),
		Phone:          rec.GetString("phone"),
		Email:          rec.GetString("email"),
		CPF:            rec.GetString("cpf"),
		OriginID:       rec.GetString("origin_id"),
		ApprovalStatus: rec.GetString("approval_status"),
		CreatedAt:      parseTime(rec.GetString("created_at")),
		UpdatedAt:      parseTime(rec.GetString("updated_at")),
	}
}

func (r *patientRepo) Create(ctx context.Context, p *patient.Patient) error {
	col, err := r.app.FindCollectionByNameOrId("patient")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *patientRepo) Update(ctx context.Context, p *patient.Patient) error {
	rec, err := getRecordByID(r.app, "patient", p.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *patientRepo) Get(ctx context.Context, id string) (*patient.Patient, error) {
	rec, err := r.app.FindFirstRecordByFilter("patient", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return patientFromRecord(rec), nil
}

func (r *patientRepo) GetByEmail(ctx context.Context, email string) (*patient.Patient, error) {
	rec, err := r.app.FindFirstRecordByFilter("patient", "email = {:email} && "+notDeletedFilter, dbxParams("email", email))
	if err != nil {
		return nil, mapError(err)
	}
	return patientFromRecord(rec), nil
}

func (r *patientRepo) List(ctx context.Context) ([]*patient.Patient, error) {
	recs, err := r.app.FindRecordsByFilter("patient", notDeletedFilter, "created_at", 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*patient.Patient, 0, len(recs))
	for _, rec := range recs {
		out = append(out, patientFromRecord(rec))
	}
	return out, nil
}

func (r *patientRepo) ListByApproval(ctx context.Context, status string) ([]*patient.Patient, error) {
	recs, err := r.app.FindRecordsByFilter("patient",
		"approval_status = {:status} && "+notDeletedFilter, "created_at", 0, 0,
		dbxParams("status", status))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*patient.Patient, 0, len(recs))
	for _, rec := range recs {
		out = append(out, patientFromRecord(rec))
	}
	return out, nil
}

func (r *patientRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "patient", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *patientRepo) EmailExists(ctx context.Context, email, exceptID string) (bool, error) {
	return existsByField(r.app, "patient", "email", email, exceptID)
}

func (r *patientRepo) CPFExists(ctx context.Context, cpf, exceptID string) (bool, error) {
	if cpf == "" {
		return false, nil
	}
	return existsByField(r.app, "patient", "cpf", cpf, exceptID)
}

// originRepo implementa app.OriginRepository sobre a coleção origin.
type originRepo struct{ app core.App }

var _ app.OriginRepository = (*originRepo)(nil)

// NewOriginRepository cria o repositório de origens.
func NewOriginRepository(a core.App) app.OriginRepository { return &originRepo{app: a} }

func (r *originRepo) toRecord(rec *core.Record, o *patient.Origin) {
	rec.Set("id", o.ID)
	rec.Set("name", o.Name)
	rec.Set("created_at", formatTime(o.CreatedAt))
	rec.Set("updated_at", formatTime(o.UpdatedAt))
}

func originFromRecord(rec *core.Record) *patient.Origin {
	return &patient.Origin{
		ID:        rec.Id,
		Name:      rec.GetString("name"),
		CreatedAt: parseTime(rec.GetString("created_at")),
		UpdatedAt: parseTime(rec.GetString("updated_at")),
	}
}

func (r *originRepo) Create(ctx context.Context, o *patient.Origin) error {
	col, err := r.app.FindCollectionByNameOrId("origin")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, o)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *originRepo) Update(ctx context.Context, o *patient.Origin) error {
	rec, err := getRecordByID(r.app, "origin", o.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, o)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *originRepo) Get(ctx context.Context, id string) (*patient.Origin, error) {
	rec, err := r.app.FindFirstRecordByFilter("origin", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return originFromRecord(rec), nil
}

func (r *originRepo) List(ctx context.Context) ([]*patient.Origin, error) {
	recs, err := r.app.FindRecordsByFilter("origin", notDeletedFilter, "name", 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*patient.Origin, 0, len(recs))
	for _, rec := range recs {
		out = append(out, originFromRecord(rec))
	}
	return out, nil
}

func (r *originRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "origin", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}
