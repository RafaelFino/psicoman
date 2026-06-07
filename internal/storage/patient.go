package storage

import (
	"database/sql"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListPatients() ([]domain.Patient, error) {
	rows, err := db.Query(`SELECT id, email, name, phone, birth_date, google_sub, anamnesis, created_at FROM patients ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPatients(rows)
}

func (db *DB) GetPatient(id string) (*domain.Patient, error) {
	row := db.QueryRow(`SELECT id, email, name, phone, birth_date, google_sub, anamnesis, created_at FROM patients WHERE id = ?`, id)
	return scanPatient(row)
}

func (db *DB) GetPatientByEmail(email string) (*domain.Patient, error) {
	row := db.QueryRow(`SELECT id, email, name, phone, birth_date, google_sub, anamnesis, created_at FROM patients WHERE email = ?`, email)
	return scanPatient(row)
}

func (db *DB) GetPatientByGoogleSub(sub string) (*domain.Patient, error) {
	row := db.QueryRow(`SELECT id, email, name, phone, birth_date, google_sub, anamnesis, created_at FROM patients WHERE google_sub = ?`, sub)
	return scanPatient(row)
}

func (db *DB) CreatePatient(p domain.Patient) (*domain.Patient, error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	_, err := db.Exec(
		`INSERT INTO patients (id, email, name, phone, birth_date, google_sub, anamnesis, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Email, p.Name, p.Phone, formatTimePtr(p.BirthDate), p.GoogleSub, p.Anamnesis, formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) UpdatePatient(p domain.Patient) error {
	_, err := db.Exec(
		`UPDATE patients SET name=?, phone=?, birth_date=?, anamnesis=? WHERE id=?`,
		p.Name, p.Phone, formatTimePtr(p.BirthDate), p.Anamnesis, p.ID,
	)
	return err
}

func (db *DB) UpdatePatientGoogleSub(id, sub string) error {
	_, err := db.Exec(`UPDATE patients SET google_sub=? WHERE id=?`, sub, id)
	return err
}

func scanPatient(row *sql.Row) (*domain.Patient, error) {
	var p domain.Patient
	var birthDate, createdAt sql.NullString
	err := row.Scan(&p.ID, &p.Email, &p.Name, &p.Phone, &birthDate, &p.GoogleSub, &p.Anamnesis, &createdAt)
	if err != nil {
		return nil, err
	}
	p.BirthDate = parseTimePtr(birthDate)
	p.CreatedAt = parseTime(createdAt.String)
	return &p, nil
}

func scanPatients(rows *sql.Rows) ([]domain.Patient, error) {
	var list []domain.Patient
	for rows.Next() {
		var p domain.Patient
		var birthDate, createdAt sql.NullString
		if err := rows.Scan(&p.ID, &p.Email, &p.Name, &p.Phone, &birthDate, &p.GoogleSub, &p.Anamnesis, &createdAt); err != nil {
			return nil, err
		}
		p.BirthDate = parseTimePtr(birthDate)
		p.CreatedAt = parseTime(createdAt.String)
		list = append(list, p)
	}
	return list, rows.Err()
}
