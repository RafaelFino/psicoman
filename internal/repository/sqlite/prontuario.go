package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// ProntuarioRepo implementa service.ProntuarioRepository sobre SQLite.
type ProntuarioRepo struct{ db *sql.DB }

// NewProntuarioRepo cria o repositório de prontuário.
func NewProntuarioRepo(db *DB) *ProntuarioRepo { return &ProntuarioRepo{db: db.DB} }

var _ service.ProntuarioRepository = (*ProntuarioRepo)(nil)

// GetAnamnesis busca a anamnese de um paciente.
func (r *ProntuarioRepo) GetAnamnesis(ctx context.Context, patientID string) (*domain.Anamnesis, error) {
	var (
		a                  domain.Anamnesis
		createdAt, updated string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, patient_id, content, created_at, updated_at FROM anamnesis WHERE patient_id=?`, patientID).
		Scan(&a.ID, &a.PatientID, &a.Content, &createdAt, &updated)
	if err != nil {
		return nil, mapError(err)
	}
	if t, err := clock.Parse(createdAt); err == nil {
		a.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		a.UpdatedAt = t
	}
	return &a, nil
}

// UpsertAnamnesis cria ou atualiza a anamnese (uma por paciente).
func (r *ProntuarioRepo) UpsertAnamnesis(ctx context.Context, a *domain.Anamnesis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO anamnesis (id, patient_id, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(patient_id) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		a.ID, a.PatientID, a.Content, clock.Format(a.CreatedAt), clock.Format(a.UpdatedAt))
	return mapError(err)
}

// CreateNote insere uma nota.
func (r *ProntuarioRepo) CreateNote(ctx context.Context, n *domain.Note) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO note (id, patient_id, session_id, content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		n.ID, n.PatientID, nullStr(n.SessionID), n.Content,
		clock.Format(n.CreatedAt), clock.Format(n.UpdatedAt))
	return mapError(err)
}

// ListNotes devolve as notas do paciente ordenadas por created_at (crescente).
func (r *ProntuarioRepo) ListNotes(ctx context.Context, patientID string) ([]*domain.Note, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, patient_id, session_id, content, created_at, updated_at
		 FROM note WHERE patient_id=? AND deleted_at IS NULL ORDER BY created_at`, patientID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Note
	for rows.Next() {
		var (
			n                  domain.Note
			sessionID          sql.NullString
			createdAt, updated string
		)
		if err := rows.Scan(&n.ID, &n.PatientID, &sessionID, &n.Content, &createdAt, &updated); err != nil {
			return nil, err
		}
		n.SessionID = sessionID.String
		if t, err := clock.Parse(createdAt); err == nil {
			n.CreatedAt = t
		}
		if t, err := clock.Parse(updated); err == nil {
			n.UpdatedAt = t
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

// DeleteNote faz soft-delete de uma nota.
func (r *ProntuarioRepo) DeleteNote(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE note SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}
