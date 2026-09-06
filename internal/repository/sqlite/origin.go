package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// OriginRepo implementa service.OriginRepository sobre SQLite.
type OriginRepo struct{ db *sql.DB }

// NewOriginRepo cria o repositório de origens.
func NewOriginRepo(db *DB) *OriginRepo { return &OriginRepo{db: db.DB} }

var _ service.OriginRepository = (*OriginRepo)(nil)

// Create insere uma origem.
func (r *OriginRepo) Create(ctx context.Context, o *domain.Origin) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO origin (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		o.ID, o.Name, clock.Format(o.CreatedAt), clock.Format(o.UpdatedAt))
	return mapError(err)
}

// Update atualiza uma origem.
func (r *OriginRepo) Update(ctx context.Context, o *domain.Origin) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE origin SET name=?, updated_at=? WHERE id=? AND deleted_at IS NULL`,
		o.Name, clock.Format(o.UpdatedAt), o.ID)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// Get busca uma origem ativa por id.
func (r *OriginRepo) Get(ctx context.Context, id string) (*domain.Origin, error) {
	var (
		o                  domain.Origin
		createdAt, updated string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM origin WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&o.ID, &o.Name, &createdAt, &updated)
	if err != nil {
		return nil, mapError(err)
	}
	if t, err := clock.Parse(createdAt); err == nil {
		o.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		o.UpdatedAt = t
	}
	return &o, nil
}

// List devolve as origens ativas.
func (r *OriginRepo) List(ctx context.Context) ([]*domain.Origin, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, created_at, updated_at FROM origin WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Origin
	for rows.Next() {
		var (
			o                  domain.Origin
			createdAt, updated string
		)
		if err := rows.Scan(&o.ID, &o.Name, &createdAt, &updated); err != nil {
			return nil, err
		}
		if t, err := clock.Parse(createdAt); err == nil {
			o.CreatedAt = t
		}
		if t, err := clock.Parse(updated); err == nil {
			o.UpdatedAt = t
		}
		out = append(out, &o)
	}
	return out, rows.Err()
}

// SoftDelete marca a origem como removida.
func (r *OriginRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE origin SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}
