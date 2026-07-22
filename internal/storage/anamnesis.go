package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListAnamnesisTemplates() ([]domain.AnamnesisTemplate, error) {
	rows, err := db.Query(`SELECT id, name, target_age_group, fields_json, is_active, created_at, updated_at
		FROM anamnesis_templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.AnamnesisTemplate
	for rows.Next() {
		t, err := scanAnamnesisTemplateRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *t)
	}
	return list, rows.Err()
}

func (db *DB) GetAnamnesisTemplate(id string) (*domain.AnamnesisTemplate, error) {
	row := db.QueryRow(`SELECT id, name, target_age_group, fields_json, is_active, created_at, updated_at
		FROM anamnesis_templates WHERE id = ?`, id)
	var t domain.AnamnesisTemplate
	var fieldsJSON, createdAt, updatedAt string
	var isActive int
	err := row.Scan(&t.ID, &t.Name, &t.TargetAgeGroup, &fieldsJSON, &isActive, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	t.IsActive = isActive == 1
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	_ = json.Unmarshal([]byte(fieldsJSON), &t.Fields)
	return &t, nil
}

func (db *DB) CreateAnamnesisTemplate(t domain.AnamnesisTemplate) (*domain.AnamnesisTemplate, error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	fieldsJSON, _ := json.Marshal(t.Fields)
	_, err := db.Exec(
		`INSERT INTO anamnesis_templates (id, name, target_age_group, fields_json, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.TargetAgeGroup, string(fieldsJSON), boolInt(t.IsActive), formatTime(now), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) UpdateAnamnesisTemplate(t domain.AnamnesisTemplate) error {
	t.UpdatedAt = time.Now().UTC()
	fieldsJSON, _ := json.Marshal(t.Fields)
	_, err := db.Exec(
		`UPDATE anamnesis_templates SET name=?, target_age_group=?, fields_json=?, is_active=?, updated_at=? WHERE id=?`,
		t.Name, t.TargetAgeGroup, string(fieldsJSON), boolInt(t.IsActive), formatTime(t.UpdatedAt), t.ID,
	)
	return err
}

func (db *DB) DeleteAnamnesisTemplate(id string) error {
	_, err := db.Exec(`DELETE FROM anamnesis_templates WHERE id=?`, id)
	return err
}

func (db *DB) ListAnamnesisResponses(patientID string) ([]domain.AnamnesisResponse, error) {
	q := `SELECT ar.id, ar.patient_id, p.name, ar.template_id, at.name,
		ar.responses_json, ar.completed_at, ar.created_at
		FROM anamnesis_responses ar
		JOIN patients p ON p.id = ar.patient_id
		JOIN anamnesis_templates at ON at.id = ar.template_id`
	args := []any{}
	if patientID != "" {
		q += ` WHERE ar.patient_id = ?`
		args = append(args, patientID)
	}
	q += ` ORDER BY ar.created_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnamnesisResponses(rows)
}

func (db *DB) GetAnamnesisResponse(id string) (*domain.AnamnesisResponse, error) {
	row := db.QueryRow(`SELECT ar.id, ar.patient_id, p.name, ar.template_id, at.name,
		ar.responses_json, ar.completed_at, ar.created_at
		FROM anamnesis_responses ar
		JOIN patients p ON p.id = ar.patient_id
		JOIN anamnesis_templates at ON at.id = ar.template_id
		WHERE ar.id = ?`, id)
	return scanAnamnesisResponse(row)
}

func (db *DB) CreateAnamnesisResponse(r domain.AnamnesisResponse) (*domain.AnamnesisResponse, error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	responsesJSON, _ := json.Marshal(r.Responses)
	_, err := db.Exec(
		`INSERT INTO anamnesis_responses (id, patient_id, template_id, responses_json, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.PatientID, r.TemplateID, string(responsesJSON), formatTimePtr(r.CompletedAt), formatTime(now),
	)
	if err != nil {
		return nil, err
	}
	return db.GetAnamnesisResponse(r.ID)
}

func scanAnamnesisTemplateRow(rows *sql.Rows) (*domain.AnamnesisTemplate, error) {
	var t domain.AnamnesisTemplate
	var fieldsJSON, createdAt, updatedAt string
	var isActive int
	err := rows.Scan(&t.ID, &t.Name, &t.TargetAgeGroup, &fieldsJSON, &isActive, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	t.IsActive = isActive == 1
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	_ = json.Unmarshal([]byte(fieldsJSON), &t.Fields)
	return &t, nil
}

func scanAnamnesisResponse(row *sql.Row) (*domain.AnamnesisResponse, error) {
	var r domain.AnamnesisResponse
	var responsesJSON, createdAt string
	var completedAt sql.NullString
	err := row.Scan(&r.ID, &r.PatientID, &r.PatientName, &r.TemplateID, &r.TemplateName,
		&responsesJSON, &completedAt, &createdAt)
	if err != nil {
		return nil, err
	}
	r.CreatedAt = parseTime(createdAt)
	r.CompletedAt = parseTimePtr(completedAt)
	r.Responses = map[string]string{}
	_ = json.Unmarshal([]byte(responsesJSON), &r.Responses)
	return &r, nil
}

func scanAnamnesisResponses(rows *sql.Rows) ([]domain.AnamnesisResponse, error) {
	var list []domain.AnamnesisResponse
	for rows.Next() {
		var r domain.AnamnesisResponse
		var responsesJSON, createdAt string
		var completedAt sql.NullString
		if err := rows.Scan(&r.ID, &r.PatientID, &r.PatientName, &r.TemplateID, &r.TemplateName,
			&responsesJSON, &completedAt, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt = parseTime(createdAt)
		r.CompletedAt = parseTimePtr(completedAt)
		r.Responses = map[string]string{}
		_ = json.Unmarshal([]byte(responsesJSON), &r.Responses)
		list = append(list, r)
	}
	return list, rows.Err()
}
