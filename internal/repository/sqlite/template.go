package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// TemplateRepo implementa service.TemplateRepository sobre SQLite.
type TemplateRepo struct{ db *sql.DB }

// NewTemplateRepo cria o repositório de templates.
func NewTemplateRepo(db *DB) *TemplateRepo { return &TemplateRepo{db: db.DB} }

var _ service.TemplateRepository = (*TemplateRepo)(nil)

// Create insere um template.
func (r *TemplateRepo) Create(ctx context.Context, t *domain.Template) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO template (id, name, body_md, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.BodyMD, clock.Format(t.CreatedAt), clock.Format(t.UpdatedAt))
	return mapError(err)
}

// Get busca um template ativo por id.
func (r *TemplateRepo) Get(ctx context.Context, id string) (*domain.Template, error) {
	var (
		t                  domain.Template
		createdAt, updated string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, body_md, created_at, updated_at FROM template WHERE id=? AND deleted_at IS NULL`, id).
		Scan(&t.ID, &t.Name, &t.BodyMD, &createdAt, &updated)
	if err != nil {
		return nil, mapError(err)
	}
	if ts, err := clock.Parse(createdAt); err == nil {
		t.CreatedAt = ts
	}
	if ts, err := clock.Parse(updated); err == nil {
		t.UpdatedAt = ts
	}
	return &t, nil
}

// List devolve os templates ativos.
func (r *TemplateRepo) List(ctx context.Context) ([]*domain.Template, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, body_md, created_at, updated_at FROM template WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Template
	for rows.Next() {
		var (
			t                  domain.Template
			createdAt, updated string
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.BodyMD, &createdAt, &updated); err != nil {
			return nil, err
		}
		if ts, err := clock.Parse(createdAt); err == nil {
			t.CreatedAt = ts
		}
		if ts, err := clock.Parse(updated); err == nil {
			t.UpdatedAt = ts
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// SoftDelete marca um template como removido.
func (r *TemplateRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE template SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

// RecordSend registra um envio de template.
func (r *TemplateRepo) RecordSend(ctx context.Context, ts *domain.TemplateSend) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO template_send (id, template_id, patient_id, rendered_html, sent_at)
		 VALUES (?, ?, ?, ?, ?)`,
		ts.ID, ts.TemplateID, ts.PatientID, ts.RenderedHTML, clock.Format(ts.SentAt))
	return mapError(err)
}
