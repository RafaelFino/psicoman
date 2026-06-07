package storage

import (
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListDocuments(patientID string) ([]domain.Document, error) {
	q := `SELECT id, patient_id, appointment_id, filename, mime_type, path, uploaded_by, doc_type, created_at FROM documents`
	var rows interface{ Scan(...any) error }
	var err error
	if patientID != "" {
		r, e := db.Query(q+` WHERE patient_id=? ORDER BY created_at DESC`, patientID)
		err = e
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return scanDocumentRows(r)
	}
	r, e := db.Query(q + ` ORDER BY created_at DESC`)
	err = e
	if err != nil {
		return nil, err
	}
	defer r.Close()
	_ = rows
	return scanDocumentRows(r)
}

func (db *DB) GetDocument(id string) (*domain.Document, error) {
	row := db.QueryRow(`SELECT id, patient_id, appointment_id, filename, mime_type, path, uploaded_by, doc_type, created_at FROM documents WHERE id=?`, id)
	var d domain.Document
	var createdAt string
	err := row.Scan(&d.ID, &d.PatientID, &d.AppointmentID, &d.Filename, &d.MimeType, &d.Path, &d.UploadedBy, &d.DocType, &createdAt)
	if err != nil {
		return nil, err
	}
	d.CreatedAt = parseTime(createdAt)
	return &d, nil
}

func (db *DB) CreateDocument(d domain.Document) (*domain.Document, error) {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO documents (id, patient_id, appointment_id, filename, mime_type, path, uploaded_by, doc_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.PatientID, d.AppointmentID, d.Filename, d.MimeType, d.Path, d.UploadedBy, d.DocType, formatTime(d.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func scanDocumentRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]domain.Document, error) {
	var list []domain.Document
	for rows.Next() {
		var d domain.Document
		var createdAt string
		if err := rows.Scan(&d.ID, &d.PatientID, &d.AppointmentID, &d.Filename, &d.MimeType, &d.Path, &d.UploadedBy, &d.DocType, &createdAt); err != nil {
			return nil, err
		}
		d.CreatedAt = parseTime(createdAt)
		list = append(list, d)
	}
	return list, rows.Err()
}
