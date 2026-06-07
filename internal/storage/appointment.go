package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListAppointments(from, to time.Time, patientID string) ([]domain.Appointment, error) {
	q := `SELECT a.id, a.patient_id, p.name, a.type, a.status, a.scheduled_at, a.duration_minutes,
		a.google_event_id, a.meet_link, a.notes, a.report_html, a.cancellation_reason, a.created_at, a.updated_at
		FROM appointments a JOIN patients p ON p.id = a.patient_id
		WHERE a.scheduled_at >= ? AND a.scheduled_at < ?`
	args := []any{formatTime(from), formatTime(to)}

	if patientID != "" {
		q += ` AND a.patient_id = ?`
		args = append(args, patientID)
	}
	q += ` ORDER BY a.scheduled_at`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAppointments(rows)
}

func (db *DB) GetAppointment(id string) (*domain.Appointment, error) {
	row := db.QueryRow(`SELECT a.id, a.patient_id, p.name, a.type, a.status, a.scheduled_at, a.duration_minutes,
		a.google_event_id, a.meet_link, a.notes, a.report_html, a.cancellation_reason, a.created_at, a.updated_at
		FROM appointments a JOIN patients p ON p.id = a.patient_id WHERE a.id = ?`, id)
	return scanAppointment(row)
}

func (db *DB) CreateAppointment(a domain.Appointment) (*domain.Appointment, error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.DurationMinutes == 0 {
		a.DurationMinutes = 50
	}
	_, err := db.Exec(
		`INSERT INTO appointments (id, patient_id, type, status, scheduled_at, duration_minutes, google_event_id, meet_link, notes, report_html, cancellation_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.PatientID, a.Type, a.Status, formatTime(a.ScheduledAt), a.DurationMinutes,
		a.GoogleEventID, a.MeetLink, a.Notes, a.ReportHTML, a.CancellationReason, formatTime(now), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return db.GetAppointment(a.ID)
}

func (db *DB) UpdateAppointment(a domain.Appointment) error {
	a.UpdatedAt = time.Now().UTC()
	_, err := db.Exec(
		`UPDATE appointments SET type=?, status=?, scheduled_at=?, duration_minutes=?, google_event_id=?, meet_link=?, notes=?, report_html=?, cancellation_reason=?, updated_at=? WHERE id=?`,
		a.Type, a.Status, formatTime(a.ScheduledAt), a.DurationMinutes, a.GoogleEventID, a.MeetLink,
		a.Notes, a.ReportHTML, a.CancellationReason, formatTime(a.UpdatedAt), a.ID,
	)
	return err
}

func (db *DB) CountReschedulesThisMonth(patientID string, month, year int) (int, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM appointments WHERE patient_id=? AND status=? AND updated_at >= ? AND updated_at < ?`,
		patientID, domain.StatusRescheduled, formatTime(start), formatTime(end),
	).Scan(&count)
	return count, err
}

func (db *DB) ListCompletedInMonth(month, year int) ([]domain.Appointment, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return db.ListAppointments(start, end, "")
}

func scanAppointment(row *sql.Row) (*domain.Appointment, error) {
	var a domain.Appointment
	var scheduledAt, createdAt, updatedAt string
	err := row.Scan(&a.ID, &a.PatientID, &a.PatientName, &a.Type, &a.Status, &scheduledAt, &a.DurationMinutes,
		&a.GoogleEventID, &a.MeetLink, &a.Notes, &a.ReportHTML, &a.CancellationReason, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	a.ScheduledAt = parseTime(scheduledAt)
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return &a, nil
}

func scanAppointments(rows *sql.Rows) ([]domain.Appointment, error) {
	var list []domain.Appointment
	for rows.Next() {
		var a domain.Appointment
		var scheduledAt, createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.PatientID, &a.PatientName, &a.Type, &a.Status, &scheduledAt, &a.DurationMinutes,
			&a.GoogleEventID, &a.MeetLink, &a.Notes, &a.ReportHTML, &a.CancellationReason, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.ScheduledAt = parseTime(scheduledAt)
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		list = append(list, a)
	}
	return list, rows.Err()
}

func (db *DB) HasConflict(scheduledAt time.Time, duration int, excludeID string) (bool, error) {
	end := scheduledAt.Add(time.Duration(duration) * time.Minute)
	rows, err := db.Query(
		`SELECT scheduled_at, duration_minutes FROM appointments WHERE status IN (?, ?) AND id != ?`,
		domain.StatusScheduled, domain.StatusRescheduled, excludeID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var at string
		var dur int
		if err := rows.Scan(&at, &dur); err != nil {
			return false, err
		}
		existing := parseTime(at)
		existingEnd := existing.Add(time.Duration(dur) * time.Minute)
		if scheduledAt.Before(existingEnd) && end.After(existing) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseTimePtr(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}

func monthYearKey(month, year int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}
