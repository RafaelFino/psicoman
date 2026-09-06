package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// ReportRepo implementa service.ReportRepository sobre SQLite.
type ReportRepo struct{ db *sql.DB }

// NewReportRepo cria o repositório de relatórios.
func NewReportRepo(db *DB) *ReportRepo { return &ReportRepo{db: db.DB} }

var _ service.ReportRepository = (*ReportRepo)(nil)

// RevenueByOrigin soma os pagamentos recebidos no período, agrupados pela
// origem do paciente do débito quitado.
func (r *ReportRepo) RevenueByOrigin(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(pt.origin_id, ''), COALESCE(SUM(pay.amount), 0)
		 FROM payment pay
		 JOIN debt d ON d.id = pay.debt_id
		 JOIN patient pt ON pt.id = d.patient_id
		 WHERE pay.paid_at >= ? AND pay.paid_at < ?
		 GROUP BY pt.origin_id`,
		clock.Format(de), clock.Format(ate))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var (
			originID string
			sum      int64
		)
		if err := rows.Scan(&originID, &sum); err != nil {
			return nil, err
		}
		out[originID] = sum
	}
	return out, rows.Err()
}

// DebtsSummary agrega débitos gerados, em aberto, recebidos e vencidos.
func (r *ReportRepo) DebtsSummary(ctx context.Context, de, ate time.Time) (*service.FinancialSummary, error) {
	deStr, ateStr := clock.Format(de), clock.Format(ate)
	sum := &service.FinancialSummary{}

	// Gerados no período.
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM debt
		 WHERE deleted_at IS NULL AND created_at >= ? AND created_at < ?`,
		deStr, ateStr).Scan(&sum.Generated); err != nil {
		return nil, mapError(err)
	}

	// Em aberto (status aberto ou parcial), gerados no período.
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM debt
		 WHERE deleted_at IS NULL AND status IN ('aberto','parcial')
		 AND created_at >= ? AND created_at < ?`,
		deStr, ateStr).Scan(&sum.Open); err != nil {
		return nil, mapError(err)
	}

	// Recebido no período (pagamentos).
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM payment WHERE paid_at >= ? AND paid_at < ?`,
		deStr, ateStr).Scan(&sum.Received); err != nil {
		return nil, mapError(err)
	}

	// Vencido e não quitado (due_date passou e status != pago), no período de geração.
	now := clock.Format(clock.Now())
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM debt
		 WHERE deleted_at IS NULL AND status IN ('aberto','parcial')
		 AND due_date IS NOT NULL AND due_date < ?
		 AND created_at >= ? AND created_at < ?`,
		now, deStr, ateStr).Scan(&sum.Overdue); err != nil {
		return nil, mapError(err)
	}

	return sum, nil
}

// CostTotalByKind soma os custos periodizados por categoria no intervalo.
func (r *ReportRepo) CostTotalByKind(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT cc.kind, ci.amount, ci.period
		 FROM cost_item ci JOIN cost_category cc ON cc.id = ci.category_id
		 WHERE ci.deleted_at IS NULL`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	days := ate.Sub(de).Hours() / 24
	if days < 1 {
		days = 1
	}
	out := map[string]int64{}
	for rows.Next() {
		var (
			kind   string
			amount int64
			period string
		)
		if err := rows.Scan(&kind, &amount, &period); err != nil {
			return nil, err
		}
		out[kind] += periodize(amount, period, days)
	}
	return out, rows.Err()
}

// SessionCostByPatient soma o custo atribuído às sessões de cada paciente.
func (r *ReportRepo) SessionCostByPatient(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT s.patient_id, COALESCE(SUM(sc.amount),0)
		 FROM session_cost sc JOIN session s ON s.id = sc.session_id
		 WHERE s.starts_at >= ? AND s.starts_at < ?
		 GROUP BY s.patient_id`,
		clock.Format(de), clock.Format(ate))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var (
			patientID string
			sum       int64
		)
		if err := rows.Scan(&patientID, &sum); err != nil {
			return nil, err
		}
		out[patientID] = sum
	}
	return out, rows.Err()
}

// periodize converte um custo periódico para o intervalo de `days` dias.
func periodize(amount int64, period string, days float64) int64 {
	switch period {
	case "diario":
		return int64(float64(amount) * days)
	case "mensal":
		return int64(float64(amount) * days / 30.0)
	case "anual":
		return int64(float64(amount) * days / 365.0)
	default:
		return amount
	}
}
