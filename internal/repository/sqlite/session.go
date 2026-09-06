package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// SessionRepo implementa service.SessionRepository sobre SQLite.
type SessionRepo struct{ db *sql.DB }

// NewSessionRepo cria o repositório de sessões.
func NewSessionRepo(db *DB) *SessionRepo { return &SessionRepo{db: db.DB} }

var _ service.SessionRepository = (*SessionRepo)(nil)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Create insere uma sessão.
func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session (id, patient_id, location_id, request_id, modality, starts_at, ends_at,
		 status, bill, consider_cost, google_event_id, meet_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.PatientID, nullStr(s.LocationID), nullStr(s.RequestID), s.Modality,
		clock.Format(s.StartsAt), clock.Format(s.EndsAt), s.Status,
		boolToInt(s.Bill), boolToInt(s.ConsiderCost),
		nullStr(s.GoogleEventID), nullStr(s.MeetURL),
		clock.Format(s.CreatedAt), clock.Format(s.UpdatedAt))
	return mapError(err)
}

// Update atualiza uma sessão.
func (r *SessionRepo) Update(ctx context.Context, s *domain.Session) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE session SET location_id=?, request_id=?, modality=?, starts_at=?, ends_at=?,
		 status=?, bill=?, consider_cost=?, google_event_id=?, meet_url=?, updated_at=?
		 WHERE id=? AND deleted_at IS NULL`,
		nullStr(s.LocationID), nullStr(s.RequestID), s.Modality,
		clock.Format(s.StartsAt), clock.Format(s.EndsAt), s.Status,
		boolToInt(s.Bill), boolToInt(s.ConsiderCost),
		nullStr(s.GoogleEventID), nullStr(s.MeetURL), clock.Format(s.UpdatedAt), s.ID)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// Get busca uma sessão ativa por id.
func (r *SessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	row := r.db.QueryRowContext(ctx, sessionSelect+` WHERE id=? AND deleted_at IS NULL`, id)
	return scanSession(row)
}

// List devolve todas as sessões ativas (mais recentes primeiro).
func (r *SessionRepo) List(ctx context.Context) ([]*domain.Session, error) {
	return r.query(ctx, sessionSelect+` WHERE deleted_at IS NULL ORDER BY starts_at DESC`)
}

// ListByPatient devolve as sessões de um paciente.
func (r *SessionRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.Session, error) {
	return r.query(ctx, sessionSelect+` WHERE patient_id=? AND deleted_at IS NULL ORDER BY starts_at DESC`, patientID)
}

// SoftDelete marca uma sessão como removida.
func (r *SessionRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE session SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

const sessionSelect = `SELECT id, patient_id, location_id, request_id, modality, starts_at, ends_at,
	status, bill, consider_cost, google_event_id, meet_url, created_at, updated_at FROM session`

func (r *SessionRepo) query(ctx context.Context, q string, args ...any) ([]*domain.Session, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Session
	for rows.Next() {
		s, err := scanSessionFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanSession(row *sql.Row) (*domain.Session, error) {
	s, err := scanSessionFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return s, nil
}

func scanSessionFrom(sc scanner) (*domain.Session, error) {
	var (
		s                     domain.Session
		locationID, requestID sql.NullString
		eventID, meetURL      sql.NullString
		bill, considerCost    int
		startsAt, endsAt      string
		createdAt, updated    string
	)
	if err := sc.Scan(&s.ID, &s.PatientID, &locationID, &requestID, &s.Modality,
		&startsAt, &endsAt, &s.Status, &bill, &considerCost, &eventID, &meetURL,
		&createdAt, &updated); err != nil {
		return nil, err
	}
	s.LocationID = locationID.String
	s.RequestID = requestID.String
	s.GoogleEventID = eventID.String
	s.MeetURL = meetURL.String
	s.Bill = bill == 1
	s.ConsiderCost = considerCost == 1
	if t, err := clock.Parse(startsAt); err == nil {
		s.StartsAt = t
	}
	if t, err := clock.Parse(endsAt); err == nil {
		s.EndsAt = t
	}
	if t, err := clock.Parse(createdAt); err == nil {
		s.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		s.UpdatedAt = t
	}
	return &s, nil
}
