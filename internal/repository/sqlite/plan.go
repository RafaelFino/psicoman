package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PlanRepo implementa service.PlanRepository sobre SQLite.
type PlanRepo struct{ db *sql.DB }

// NewPlanRepo cria o repositório de planos.
func NewPlanRepo(db *DB) *PlanRepo { return &PlanRepo{db: db.DB} }

var _ service.PlanRepository = (*PlanRepo)(nil)

const planSelect = `SELECT id, patient_id, type, amount, starts_at, ends_at, created_at, updated_at FROM plan`

// Create insere um plano.
func (r *PlanRepo) Create(ctx context.Context, p *domain.Plan) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO plan (id, patient_id, type, amount, starts_at, ends_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.PatientID, p.Type, p.Amount, clock.Format(p.StartsAt),
		nullTime(p.EndsAt), clock.Format(p.CreatedAt), clock.Format(p.UpdatedAt))
	return mapError(err)
}

// Update atualiza um plano.
func (r *PlanRepo) Update(ctx context.Context, p *domain.Plan) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE plan SET type=?, amount=?, starts_at=?, ends_at=?, updated_at=?
		 WHERE id=? AND deleted_at IS NULL`,
		p.Type, p.Amount, clock.Format(p.StartsAt), nullTime(p.EndsAt),
		clock.Format(p.UpdatedAt), p.ID)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// Get busca um plano ativo por id.
func (r *PlanRepo) Get(ctx context.Context, id string) (*domain.Plan, error) {
	row := r.db.QueryRowContext(ctx, planSelect+` WHERE id=? AND deleted_at IS NULL`, id)
	return scanPlan(row)
}

// GetActiveByPatient devolve o plano vigente do paciente em ref (o mais recente).
func (r *PlanRepo) GetActiveByPatient(ctx context.Context, patientID string, ref time.Time) (*domain.Plan, error) {
	refStr := clock.Format(ref)
	row := r.db.QueryRowContext(ctx,
		planSelect+` WHERE patient_id=? AND deleted_at IS NULL
		 AND starts_at <= ? AND (ends_at IS NULL OR ends_at >= ?)
		 ORDER BY starts_at DESC LIMIT 1`,
		patientID, refStr, refStr)
	return scanPlan(row)
}

// ListByPatient devolve os planos de um paciente.
func (r *PlanRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.Plan, error) {
	return r.query(ctx, planSelect+` WHERE patient_id=? AND deleted_at IS NULL ORDER BY starts_at DESC`, patientID)
}

// ListFixedCycleActive devolve planos fechados vigentes em ref.
func (r *PlanRepo) ListFixedCycleActive(ctx context.Context, ref time.Time) ([]*domain.Plan, error) {
	refStr := clock.Format(ref)
	return r.query(ctx,
		planSelect+` WHERE type IN ('plano_fechado_mensal','plano_fechado_trimestral')
		 AND deleted_at IS NULL AND starts_at <= ? AND (ends_at IS NULL OR ends_at >= ?)`,
		refStr, refStr)
}

// SoftDelete marca um plano como removido.
func (r *PlanRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plan SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

func (r *PlanRepo) query(ctx context.Context, q string, args ...any) ([]*domain.Plan, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Plan
	for rows.Next() {
		p, err := scanPlanFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPlan(row *sql.Row) (*domain.Plan, error) {
	p, err := scanPlanFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return p, nil
}

func scanPlanFrom(sc scanner) (*domain.Plan, error) {
	var (
		p                  domain.Plan
		endsAt             sql.NullString
		startsAt           string
		createdAt, updated string
	)
	if err := sc.Scan(&p.ID, &p.PatientID, &p.Type, &p.Amount, &startsAt, &endsAt,
		&createdAt, &updated); err != nil {
		return nil, err
	}
	if t, err := clock.Parse(startsAt); err == nil {
		p.StartsAt = t
	}
	if endsAt.Valid && endsAt.String != "" {
		if t, err := clock.Parse(endsAt.String); err == nil {
			p.EndsAt = t
		}
	}
	if t, err := clock.Parse(createdAt); err == nil {
		p.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		p.UpdatedAt = t
	}
	return &p, nil
}

// nullTime converte um time zero em NULL.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return clock.Format(t)
}
