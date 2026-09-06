package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// DebtRepo implementa service.DebtRepository sobre SQLite.
type DebtRepo struct{ db *sql.DB }

// NewDebtRepo cria o repositório de débitos.
func NewDebtRepo(db *DB) *DebtRepo { return &DebtRepo{db: db.DB} }

var _ service.DebtRepository = (*DebtRepo)(nil)

const debtSelect = `SELECT id, patient_id, session_id, plan_id, billing_period, amount, due_date,
	status, idempotency_key, pdf_ged_file_id, created_at, updated_at FROM debt`

// InsertIfAbsent grava o débito só se a idempotency_key ainda não existir.
// Usa ON CONFLICT DO NOTHING (idempotência sem race entre check e insert).
func (r *DebtRepo) InsertIfAbsent(ctx context.Context, d *domain.Debt) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO debt (id, patient_id, session_id, plan_id, billing_period, amount, due_date,
		 status, idempotency_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(idempotency_key) DO NOTHING`,
		d.ID, d.PatientID, nullStr(d.SessionID), nullStr(d.PlanID), nullStr(d.BillingPeriod),
		d.Amount, nullTime(d.DueDate), d.Status, d.IdempotencyKey,
		clock.Format(d.CreatedAt), clock.Format(d.UpdatedAt))
	if err != nil {
		return false, mapError(err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Get busca um débito por id.
func (r *DebtRepo) Get(ctx context.Context, id string) (*domain.Debt, error) {
	row := r.db.QueryRowContext(ctx, debtSelect+` WHERE id=? AND deleted_at IS NULL`, id)
	return scanDebt(row)
}

// GetByIdempotencyKey busca um débito pela chave de idempotência.
func (r *DebtRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Debt, error) {
	row := r.db.QueryRowContext(ctx, debtSelect+` WHERE idempotency_key=? AND deleted_at IS NULL`, key)
	return scanDebt(row)
}

// List devolve todos os débitos ativos.
func (r *DebtRepo) List(ctx context.Context) ([]*domain.Debt, error) {
	return r.query(ctx, debtSelect+` WHERE deleted_at IS NULL ORDER BY created_at DESC`)
}

// ListByPatient devolve os débitos de um paciente.
func (r *DebtRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.Debt, error) {
	return r.query(ctx, debtSelect+` WHERE patient_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, patientID)
}

// UpdateStatus atualiza o status de um débito.
func (r *DebtRepo) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE debt SET status=?, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		status, clock.Format(clock.Now()), id)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// SetPDF vincula o PDF de cobrança (ged_file) ao débito.
func (r *DebtRepo) SetPDF(ctx context.Context, id, pdfFileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE debt SET pdf_ged_file_id=?, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		pdfFileID, clock.Format(clock.Now()), id)
	return mapError(err)
}

func (r *DebtRepo) query(ctx context.Context, q string, args ...any) ([]*domain.Debt, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Debt
	for rows.Next() {
		d, err := scanDebtFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanDebt(row *sql.Row) (*domain.Debt, error) {
	d, err := scanDebtFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return d, nil
}

func scanDebtFrom(sc scanner) (*domain.Debt, error) {
	var (
		d                  domain.Debt
		sessionID, planID  sql.NullString
		billingPeriod, pdf sql.NullString
		dueDate            sql.NullString
		createdAt, updated string
	)
	if err := sc.Scan(&d.ID, &d.PatientID, &sessionID, &planID, &billingPeriod, &d.Amount,
		&dueDate, &d.Status, &d.IdempotencyKey, &pdf, &createdAt, &updated); err != nil {
		return nil, err
	}
	d.SessionID = sessionID.String
	d.PlanID = planID.String
	d.BillingPeriod = billingPeriod.String
	d.PDFFileID = pdf.String
	if dueDate.Valid && dueDate.String != "" {
		if t, err := clock.Parse(dueDate.String); err == nil {
			d.DueDate = t
		}
	}
	if t, err := clock.Parse(createdAt); err == nil {
		d.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		d.UpdatedAt = t
	}
	return &d, nil
}
