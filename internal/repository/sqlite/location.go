package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// LocationRepo implementa service.LocationRepository sobre SQLite.
type LocationRepo struct{ db *sql.DB }

// NewLocationRepo cria o repositório de locais.
func NewLocationRepo(db *DB) *LocationRepo { return &LocationRepo{db: db.DB} }

var _ service.LocationRepository = (*LocationRepo)(nil)

// Create insere um local.
func (r *LocationRepo) Create(ctx context.Context, l *domain.Location) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO location (id, name, address, modality, cost_amount, cost_period, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Name, nullStr(l.Address), l.Modality, l.CostAmount, l.CostPeriod,
		clock.Format(l.CreatedAt), clock.Format(l.UpdatedAt))
	return mapError(err)
}

// Update atualiza um local.
func (r *LocationRepo) Update(ctx context.Context, l *domain.Location) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE location SET name=?, address=?, modality=?, cost_amount=?, cost_period=?, updated_at=?
		 WHERE id=? AND deleted_at IS NULL`,
		l.Name, nullStr(l.Address), l.Modality, l.CostAmount, l.CostPeriod,
		clock.Format(l.UpdatedAt), l.ID)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// Get busca um local ativo por id.
func (r *LocationRepo) Get(ctx context.Context, id string) (*domain.Location, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, address, modality, cost_amount, cost_period, created_at, updated_at
		 FROM location WHERE id=? AND deleted_at IS NULL`, id)
	return scanLocation(row)
}

// List devolve os locais ativos.
func (r *LocationRepo) List(ctx context.Context) ([]*domain.Location, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, address, modality, cost_amount, cost_period, created_at, updated_at
		 FROM location WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Location
	for rows.Next() {
		l, err := scanLocationFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SoftDelete marca um local como removido.
func (r *LocationRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE location SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

// AddAvailability insere uma janela de disponibilidade.
func (r *LocationRepo) AddAvailability(ctx context.Context, a *domain.Availability) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO availability (id, location_id, weekday, start_time, end_time, capacity, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.LocationID, a.Weekday, a.StartTime, a.EndTime, a.Capacity,
		clock.Format(a.CreatedAt), clock.Format(a.UpdatedAt))
	return mapError(err)
}

// ListAvailability devolve as janelas ativas de um local.
func (r *LocationRepo) ListAvailability(ctx context.Context, locationID string) ([]*domain.Availability, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, location_id, weekday, start_time, end_time, capacity, created_at, updated_at
		 FROM availability WHERE location_id=? AND deleted_at IS NULL
		 ORDER BY weekday, start_time`, locationID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Availability
	for rows.Next() {
		var (
			a                  domain.Availability
			createdAt, updated string
		)
		if err := rows.Scan(&a.ID, &a.LocationID, &a.Weekday, &a.StartTime, &a.EndTime,
			&a.Capacity, &createdAt, &updated); err != nil {
			return nil, err
		}
		if t, err := clock.Parse(createdAt); err == nil {
			a.CreatedAt = t
		}
		if t, err := clock.Parse(updated); err == nil {
			a.UpdatedAt = t
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// DeleteAvailability marca uma janela como removida.
func (r *LocationRepo) DeleteAvailability(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE availability SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

func scanLocation(row *sql.Row) (*domain.Location, error) {
	l, err := scanLocationFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return l, nil
}

func scanLocationFrom(s scanner) (*domain.Location, error) {
	var (
		l                  domain.Location
		address            sql.NullString
		createdAt, updated string
	)
	if err := s.Scan(&l.ID, &l.Name, &address, &l.Modality, &l.CostAmount, &l.CostPeriod,
		&createdAt, &updated); err != nil {
		return nil, err
	}
	l.Address = address.String
	if t, err := clock.Parse(createdAt); err == nil {
		l.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		l.UpdatedAt = t
	}
	return &l, nil
}
