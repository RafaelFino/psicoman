package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// AuditRepo implementa service.AuditRepository sobre SQLite.
type AuditRepo struct {
	db *sql.DB
}

// NewAuditRepo cria o repositório de auditoria.
func NewAuditRepo(db *DB) *AuditRepo { return &AuditRepo{db: db.DB} }

// Insert grava uma entrada de auditoria.
func (r *AuditRepo) Insert(ctx context.Context, e *domain.AuditLog) error {
	var meta any
	if e.Metadata != nil {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
		meta = string(b)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (id, actor, action, entity, entity_id, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Actor, e.Action, nullStr(e.Entity), nullStr(e.EntityID), meta, clock.Format(e.CreatedAt),
	)
	return err
}

// List devolve as entradas mais recentes (ordem decrescente).
func (r *AuditRepo) List(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, actor, action, entity, entity_id, metadata, created_at
		 FROM audit_log ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AuditLog
	for rows.Next() {
		var (
			e                        domain.AuditLog
			entity, entityID, metaJS sql.NullString
			createdAt                string
		)
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &entity, &entityID, &metaJS, &createdAt); err != nil {
			return nil, err
		}
		e.Entity = entity.String
		e.EntityID = entityID.String
		if metaJS.Valid && metaJS.String != "" {
			_ = json.Unmarshal([]byte(metaJS.String), &e.Metadata)
		}
		if t, err := clock.Parse(createdAt); err == nil {
			e.CreatedAt = t
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// nullStr converte "" em NULL para colunas anuláveis.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
