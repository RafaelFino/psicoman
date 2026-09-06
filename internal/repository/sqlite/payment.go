package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PaymentRepo implementa service.PaymentRepository sobre SQLite.
type PaymentRepo struct{ db *sql.DB }

// NewPaymentRepo cria o repositório de pagamentos.
func NewPaymentRepo(db *DB) *PaymentRepo { return &PaymentRepo{db: db.DB} }

var _ service.PaymentRepository = (*PaymentRepo)(nil)

// Insert grava um pagamento.
func (r *PaymentRepo) Insert(ctx context.Context, p *domain.Payment) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO payment (id, debt_id, amount, paid_at, method, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.DebtID, p.Amount, clock.Format(p.PaidAt), nullStr(p.Method), clock.Format(p.CreatedAt))
	return mapError(err)
}

// ListByDebt devolve os pagamentos de um débito (mais recentes primeiro).
func (r *PaymentRepo) ListByDebt(ctx context.Context, debtID string) ([]*domain.Payment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, debt_id, amount, paid_at, method, created_at
		 FROM payment WHERE debt_id=? ORDER BY paid_at DESC`, debtID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Payment
	for rows.Next() {
		var (
			p         domain.Payment
			method    sql.NullString
			paidAt    string
			createdAt string
		)
		if err := rows.Scan(&p.ID, &p.DebtID, &p.Amount, &paidAt, &method, &createdAt); err != nil {
			return nil, err
		}
		p.Method = method.String
		if t, err := clock.Parse(paidAt); err == nil {
			p.PaidAt = t
		}
		if t, err := clock.Parse(createdAt); err == nil {
			p.CreatedAt = t
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SumByDebt devolve o total pago de um débito.
func (r *PaymentRepo) SumByDebt(ctx context.Context, debtID string) (int64, error) {
	var sum sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payment WHERE debt_id=?`, debtID).Scan(&sum)
	if err != nil {
		return 0, mapError(err)
	}
	return sum.Int64, nil
}
