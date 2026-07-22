package storage

import (
	"database/sql"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

// ─── Supervisors ─────────────────────────────────────────────────────────────

func (db *DB) ListSupervisors() ([]domain.Supervisor, error) {
	rows, err := db.Query(`SELECT id, name, email, specialty, crp, notes, created_at FROM supervisors ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Supervisor
	for rows.Next() {
		var s domain.Supervisor
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Email, &s.Specialty, &s.CRP, &s.Notes, &createdAt); err != nil {
			return nil, err
		}
		s.CreatedAt = parseTime(createdAt)
		list = append(list, s)
	}
	return list, rows.Err()
}

func (db *DB) GetSupervisor(id string) (*domain.Supervisor, error) {
	row := db.QueryRow(`SELECT id, name, email, specialty, crp, notes, created_at FROM supervisors WHERE id = ?`, id)
	var s domain.Supervisor
	var createdAt string
	err := row.Scan(&s.ID, &s.Name, &s.Email, &s.Specialty, &s.CRP, &s.Notes, &createdAt)
	if err != nil {
		return nil, err
	}
	s.CreatedAt = parseTime(createdAt)
	return &s, nil
}

func (db *DB) CreateSupervisor(s domain.Supervisor) (*domain.Supervisor, error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO supervisors (id, name, email, specialty, crp, notes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Email, s.Specialty, s.CRP, s.Notes, formatTime(s.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) UpdateSupervisor(s domain.Supervisor) error {
	_, err := db.Exec(
		`UPDATE supervisors SET name=?, email=?, specialty=?, crp=?, notes=? WHERE id=?`,
		s.Name, s.Email, s.Specialty, s.CRP, s.Notes, s.ID,
	)
	return err
}

func (db *DB) DeleteSupervisor(id string) error {
	_, err := db.Exec(`DELETE FROM supervisors WHERE id=?`, id)
	return err
}

// ─── Supervision Sessions ────────────────────────────────────────────────────

func (db *DB) ListSupervisionSessions(supervisorID string) ([]domain.SupervisionSession, error) {
	q := `SELECT ss.id, ss.supervisor_id, s.name, ss.scheduled_at, ss.duration_minutes,
		ss.notes_html, ss.topics, ss.cost_cents, ss.status, ss.created_at, ss.updated_at
		FROM supervision_sessions ss
		JOIN supervisors s ON s.id = ss.supervisor_id`
	args := []any{}
	if supervisorID != "" {
		q += ` WHERE ss.supervisor_id = ?`
		args = append(args, supervisorID)
	}
	q += ` ORDER BY ss.scheduled_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSupervisionSessions(rows)
}

func (db *DB) GetSupervisionSession(id string) (*domain.SupervisionSession, error) {
	row := db.QueryRow(`SELECT ss.id, ss.supervisor_id, s.name, ss.scheduled_at, ss.duration_minutes,
		ss.notes_html, ss.topics, ss.cost_cents, ss.status, ss.created_at, ss.updated_at
		FROM supervision_sessions ss
		JOIN supervisors s ON s.id = ss.supervisor_id
		WHERE ss.id = ?`, id)
	return scanSupervisionSession(row)
}

func (db *DB) CreateSupervisionSession(ss domain.SupervisionSession) (*domain.SupervisionSession, error) {
	if ss.ID == "" {
		ss.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	ss.CreatedAt = now
	ss.UpdatedAt = now
	if ss.DurationMinutes == 0 {
		ss.DurationMinutes = 60
	}
	if ss.Status == "" {
		ss.Status = domain.SupervisionScheduled
	}
	_, err := db.Exec(
		`INSERT INTO supervision_sessions (id, supervisor_id, scheduled_at, duration_minutes, notes_html, topics, cost_cents, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ss.ID, ss.SupervisorID, formatTime(ss.ScheduledAt), ss.DurationMinutes,
		ss.NotesHTML, ss.Topics, ss.CostCents, ss.Status, formatTime(now), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return db.GetSupervisionSession(ss.ID)
}

func (db *DB) UpdateSupervisionSession(ss domain.SupervisionSession) error {
	ss.UpdatedAt = time.Now().UTC()
	_, err := db.Exec(
		`UPDATE supervision_sessions SET scheduled_at=?, duration_minutes=?, notes_html=?, topics=?, cost_cents=?, status=?, updated_at=? WHERE id=?`,
		formatTime(ss.ScheduledAt), ss.DurationMinutes, ss.NotesHTML, ss.Topics, ss.CostCents, ss.Status, formatTime(ss.UpdatedAt), ss.ID,
	)
	return err
}

func (db *DB) SupervisionHoursForMonth(month, year int) (int, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var total int
	err := db.QueryRow(
		`SELECT COALESCE(SUM(duration_minutes), 0) FROM supervision_sessions WHERE status = ? AND scheduled_at >= ? AND scheduled_at < ?`,
		domain.SupervisionCompleted, formatTime(start), formatTime(end),
	).Scan(&total)
	return total, err
}

func scanSupervisionSession(row *sql.Row) (*domain.SupervisionSession, error) {
	var ss domain.SupervisionSession
	var scheduledAt, createdAt, updatedAt string
	err := row.Scan(&ss.ID, &ss.SupervisorID, &ss.SupervisorName, &scheduledAt, &ss.DurationMinutes,
		&ss.NotesHTML, &ss.Topics, &ss.CostCents, &ss.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	ss.ScheduledAt = parseTime(scheduledAt)
	ss.CreatedAt = parseTime(createdAt)
	ss.UpdatedAt = parseTime(updatedAt)
	return &ss, nil
}

func scanSupervisionSessions(rows *sql.Rows) ([]domain.SupervisionSession, error) {
	var list []domain.SupervisionSession
	for rows.Next() {
		var ss domain.SupervisionSession
		var scheduledAt, createdAt, updatedAt string
		if err := rows.Scan(&ss.ID, &ss.SupervisorID, &ss.SupervisorName, &scheduledAt, &ss.DurationMinutes,
			&ss.NotesHTML, &ss.Topics, &ss.CostCents, &ss.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		ss.ScheduledAt = parseTime(scheduledAt)
		ss.CreatedAt = parseTime(createdAt)
		ss.UpdatedAt = parseTime(updatedAt)
		list = append(list, ss)
	}
	return list, rows.Err()
}
