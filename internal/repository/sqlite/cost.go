package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// CostRepo implementa service.CostRepository sobre SQLite.
type CostRepo struct{ db *sql.DB }

// NewCostRepo cria o repositório de custos.
func NewCostRepo(db *DB) *CostRepo { return &CostRepo{db: db.DB} }

var _ service.CostRepository = (*CostRepo)(nil)

// CreateCategory insere uma categoria de custo.
func (r *CostRepo) CreateCategory(ctx context.Context, c *domain.CostCategory) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cost_category (id, kind, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Kind, c.Name, clock.Format(c.CreatedAt), clock.Format(c.UpdatedAt))
	return mapError(err)
}

// ListCategories devolve as categorias.
func (r *CostRepo) ListCategories(ctx context.Context) ([]*domain.CostCategory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kind, name, created_at, updated_at FROM cost_category ORDER BY kind, name`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.CostCategory
	for rows.Next() {
		var (
			c                  domain.CostCategory
			createdAt, updated string
		)
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &createdAt, &updated); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

const costItemSelect = `SELECT id, category_id, name, amount, period, origin_id, location_id, created_at, updated_at FROM cost_item`

// CreateItem insere um item de custo.
func (r *CostRepo) CreateItem(ctx context.Context, i *domain.CostItem) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cost_item (id, category_id, name, amount, period, origin_id, location_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.CategoryID, i.Name, i.Amount, i.Period, nullStr(i.OriginID), nullStr(i.LocationID),
		clock.Format(i.CreatedAt), clock.Format(i.UpdatedAt))
	return mapError(err)
}

// ListItems devolve os itens de custo ativos.
func (r *CostRepo) ListItems(ctx context.Context) ([]*domain.CostItem, error) {
	return r.queryItems(ctx, costItemSelect+` WHERE deleted_at IS NULL ORDER BY name`)
}

// ListItemsByOrigin devolve os itens de custo ligados a uma origem.
func (r *CostRepo) ListItemsByOrigin(ctx context.Context, originID string) ([]*domain.CostItem, error) {
	return r.queryItems(ctx, costItemSelect+` WHERE origin_id=? AND deleted_at IS NULL`, originID)
}

// SoftDeleteItem marca um item de custo como removido.
func (r *CostRepo) SoftDeleteItem(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE cost_item SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

func (r *CostRepo) queryItems(ctx context.Context, q string, args ...any) ([]*domain.CostItem, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.CostItem
	for rows.Next() {
		var (
			i                    domain.CostItem
			originID, locationID sql.NullString
			createdAt, updated   string
		)
		if err := rows.Scan(&i.ID, &i.CategoryID, &i.Name, &i.Amount, &i.Period,
			&originID, &locationID, &createdAt, &updated); err != nil {
			return nil, err
		}
		i.OriginID = originID.String
		i.LocationID = locationID.String
		out = append(out, &i)
	}
	return out, rows.Err()
}

// UpsertSessionCost cria ou atualiza o custo atribuído a uma sessão.
func (r *CostRepo) UpsertSessionCost(ctx context.Context, sc *domain.SessionCost) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_cost (id, session_id, amount, method, base_snapshot, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		   amount=excluded.amount, method=excluded.method,
		   base_snapshot=excluded.base_snapshot, updated_at=excluded.updated_at`,
		sc.ID, sc.SessionID, sc.Amount, sc.Method, nullStr(sc.BaseSnapshot),
		clock.Format(sc.CreatedAt), clock.Format(sc.UpdatedAt))
	return mapError(err)
}

// GetSessionCost devolve o custo atribuído a uma sessão.
func (r *CostRepo) GetSessionCost(ctx context.Context, sessionID string) (*domain.SessionCost, error) {
	var (
		sc                 domain.SessionCost
		base               sql.NullString
		createdAt, updated string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, session_id, amount, method, base_snapshot, created_at, updated_at
		 FROM session_cost WHERE session_id=?`, sessionID).
		Scan(&sc.ID, &sc.SessionID, &sc.Amount, &sc.Method, &base, &createdAt, &updated)
	if err != nil {
		return nil, mapError(err)
	}
	sc.BaseSnapshot = base.String
	return &sc, nil
}

// CountRealizadasNoLocalNoPeriodo conta sessões realizadas que computam custo,
// no local, no intervalo [de, ate). Implementa service.CostSessionSource.
func (r *CostRepo) CountRealizadasNoLocalNoPeriodo(ctx context.Context, locationID string, de, ate time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM session
		 WHERE location_id=? AND status='realizada' AND consider_cost=1 AND deleted_at IS NULL
		 AND starts_at >= ? AND starts_at < ?`,
		locationID, clock.Format(de), clock.Format(ate)).Scan(&n)
	if err != nil {
		return 0, mapError(err)
	}
	return n, nil
}
