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

// DeleteTestData removes all patients whose name starts with "TEST " OR email ends with "@test.com",
// plus all associated data (appointments, session_notes, documents, contracts, payments, anamnesis_responses).
// Returns a map of entity type → count deleted. Wrapped in a transaction for atomicity.
func (db *DB) DeleteTestData() (map[string]int, error) {
	counts := map[string]int{
		"patients":            0,
		"appointments":       0,
		"session_notes":      0,
		"documents":          0,
		"contracts":          0,
		"payments":           0,
		"anamnesis_responses": 0,
	}

	// 1. Find patient IDs matching test criteria
	rows, err := db.Query(`SELECT id FROM patients WHERE name LIKE 'TEST %' OR email LIKE '%@test.com'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patientIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		patientIDs = append(patientIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. If no patients found, return empty counts
	if len(patientIDs) == 0 {
		return counts, nil
	}

	// 3. Build placeholder string for IN clause
	placeholders := ""
	args := make([]interface{}, len(patientIDs))
	for i, id := range patientIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	// 4. Execute deletions within a transaction
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	childTables := []struct {
		table string
		key   string
	}{
		{"session_notes", "session_notes"},
		{"documents", "documents"},
		{"contracts", "contracts"},
		{"payments", "payments"},
		{"appointments", "appointments"},
		{"anamnesis_responses", "anamnesis_responses"},
	}

	for _, ct := range childTables {
		res, err := tx.Exec("DELETE FROM "+ct.table+" WHERE patient_id IN ("+placeholders+")", args...)
		if err != nil {
			return nil, err
		}
		n, _ := res.RowsAffected()
		counts[ct.key] = int(n)
	}

	// 5. Delete the patients themselves
	res, err := tx.Exec("DELETE FROM patients WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	counts["patients"] = int(n)

	// 6. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return counts, nil
}
