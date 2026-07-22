package storage

import (
	"database/sql"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListSessionNotes(patientID string) ([]domain.SessionNote, error) {
	q := `SELECT sn.id, sn.appointment_id, sn.patient_id, p.name,
		sn.content_html, sn.private_notes,
		sn.duration_patient_min, sn.duration_analysis_min, sn.duration_admin_min,
		sn.created_at, sn.updated_at
		FROM session_notes sn JOIN patients p ON p.id = sn.patient_id`
	args := []any{}

	if patientID != "" {
		q += ` WHERE sn.patient_id = ?`
		args = append(args, patientID)
	}
	q += ` ORDER BY sn.created_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionNotes(rows)
}

func (db *DB) GetSessionNote(id string) (*domain.SessionNote, error) {
	row := db.QueryRow(`SELECT sn.id, sn.appointment_id, sn.patient_id, p.name,
		sn.content_html, sn.private_notes,
		sn.duration_patient_min, sn.duration_analysis_min, sn.duration_admin_min,
		sn.created_at, sn.updated_at
		FROM session_notes sn JOIN patients p ON p.id = sn.patient_id WHERE sn.id = ?`, id)
	return scanSessionNote(row)
}

func (db *DB) GetSessionNoteByAppointment(appointmentID string) (*domain.SessionNote, error) {
	row := db.QueryRow(`SELECT sn.id, sn.appointment_id, sn.patient_id, p.name,
		sn.content_html, sn.private_notes,
		sn.duration_patient_min, sn.duration_analysis_min, sn.duration_admin_min,
		sn.created_at, sn.updated_at
		FROM session_notes sn JOIN patients p ON p.id = sn.patient_id WHERE sn.appointment_id = ?`, appointmentID)
	return scanSessionNote(row)
}

func (db *DB) CreateSessionNote(sn domain.SessionNote) (*domain.SessionNote, error) {
	if sn.ID == "" {
		sn.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	sn.CreatedAt = now
	sn.UpdatedAt = now
	_, err := db.Exec(
		`INSERT INTO session_notes (id, appointment_id, patient_id, content_html, private_notes,
			duration_patient_min, duration_analysis_min, duration_admin_min, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sn.ID, sn.AppointmentID, sn.PatientID, sn.ContentHTML, sn.PrivateNotes,
		sn.DurationPatientMin, sn.DurationAnalysisMin, sn.DurationAdminMin,
		formatTime(now), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return db.GetSessionNote(sn.ID)
}

func (db *DB) UpdateSessionNote(sn domain.SessionNote) error {
	sn.UpdatedAt = time.Now().UTC()
	_, err := db.Exec(
		`UPDATE session_notes SET content_html=?, private_notes=?,
			duration_patient_min=?, duration_analysis_min=?, duration_admin_min=?, updated_at=?
		WHERE id=?`,
		sn.ContentHTML, sn.PrivateNotes,
		sn.DurationPatientMin, sn.DurationAnalysisMin, sn.DurationAdminMin,
		formatTime(sn.UpdatedAt), sn.ID,
	)
	return err
}

func (db *DB) SessionNoteHoursForMonth(month, year int) (patientMin, analysisMin, adminMin int, err error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	row := db.QueryRow(
		`SELECT COALESCE(SUM(duration_patient_min), 0),
			COALESCE(SUM(duration_analysis_min), 0),
			COALESCE(SUM(duration_admin_min), 0)
		FROM session_notes WHERE created_at >= ? AND created_at < ?`,
		formatTime(start), formatTime(end),
	)
	err = row.Scan(&patientMin, &analysisMin, &adminMin)
	return
}

func scanSessionNote(row *sql.Row) (*domain.SessionNote, error) {
	var sn domain.SessionNote
	var createdAt, updatedAt string
	err := row.Scan(&sn.ID, &sn.AppointmentID, &sn.PatientID, &sn.PatientName,
		&sn.ContentHTML, &sn.PrivateNotes,
		&sn.DurationPatientMin, &sn.DurationAnalysisMin, &sn.DurationAdminMin,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	sn.CreatedAt = parseTime(createdAt)
	sn.UpdatedAt = parseTime(updatedAt)
	return &sn, nil
}

func scanSessionNotes(rows *sql.Rows) ([]domain.SessionNote, error) {
	var list []domain.SessionNote
	for rows.Next() {
		var sn domain.SessionNote
		var createdAt, updatedAt string
		if err := rows.Scan(&sn.ID, &sn.AppointmentID, &sn.PatientID, &sn.PatientName,
			&sn.ContentHTML, &sn.PrivateNotes,
			&sn.DurationPatientMin, &sn.DurationAnalysisMin, &sn.DurationAdminMin,
			&createdAt, &updatedAt); err != nil {
			return nil, err
		}
		sn.CreatedAt = parseTime(createdAt)
		sn.UpdatedAt = parseTime(updatedAt)
		list = append(list, sn)
	}
	return list, rows.Err()
}
