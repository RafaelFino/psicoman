package storage

import (
	"database/sql"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListContractTemplates() ([]domain.ContractTemplate, error) {
	rows, err := db.Query(`SELECT id, name, content_html, is_active, created_at FROM contract_templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ContractTemplate
	for rows.Next() {
		var t domain.ContractTemplate
		var isActive int
		var createdAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.ContentHTML, &isActive, &createdAt); err != nil {
			return nil, err
		}
		t.IsActive = isActive == 1
		t.CreatedAt = parseTime(createdAt)
		list = append(list, t)
	}
	return list, rows.Err()
}

func (db *DB) GetContractTemplate(id string) (*domain.ContractTemplate, error) {
	row := db.QueryRow(`SELECT id, name, content_html, is_active, created_at FROM contract_templates WHERE id = ?`, id)
	var t domain.ContractTemplate
	var isActive int
	var createdAt string
	err := row.Scan(&t.ID, &t.Name, &t.ContentHTML, &isActive, &createdAt)
	if err != nil {
		return nil, err
	}
	t.IsActive = isActive == 1
	t.CreatedAt = parseTime(createdAt)
	return &t, nil
}

func (db *DB) CreateContractTemplate(t domain.ContractTemplate) (*domain.ContractTemplate, error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO contract_templates (id, name, content_html, is_active, created_at) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.ContentHTML, boolInt(t.IsActive), formatTime(t.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) UpdateContractTemplate(t domain.ContractTemplate) error {
	_, err := db.Exec(
		`UPDATE contract_templates SET name=?, content_html=?, is_active=? WHERE id=?`,
		t.Name, t.ContentHTML, boolInt(t.IsActive), t.ID,
	)
	return err
}

func (db *DB) ListContracts(patientID string) ([]domain.Contract, error) {
	q := `SELECT c.id, c.patient_id, p.name, c.template_id, ct.name,
		c.status, c.generated_html, c.signed_at, c.signature_ip, c.signature_user_agent, c.pdf_path, c.created_at
		FROM contracts c
		JOIN patients p ON p.id = c.patient_id
		JOIN contract_templates ct ON ct.id = c.template_id`
	args := []any{}
	if patientID != "" {
		q += ` WHERE c.patient_id = ?`
		args = append(args, patientID)
	}
	q += ` ORDER BY c.created_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContracts(rows)
}

func (db *DB) GetContract(id string) (*domain.Contract, error) {
	row := db.QueryRow(`SELECT c.id, c.patient_id, p.name, c.template_id, ct.name,
		c.status, c.generated_html, c.signed_at, c.signature_ip, c.signature_user_agent, c.pdf_path, c.created_at
		FROM contracts c
		JOIN patients p ON p.id = c.patient_id
		JOIN contract_templates ct ON ct.id = c.template_id
		WHERE c.id = ?`, id)
	return scanContract(row)
}

func (db *DB) CreateContract(ct domain.Contract) (*domain.Contract, error) {
	if ct.ID == "" {
		ct.ID = uuid.New().String()
	}
	ct.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO contracts (id, patient_id, template_id, status, generated_html, signed_at, signature_ip, signature_user_agent, pdf_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ct.ID, ct.PatientID, ct.TemplateID, ct.Status, ct.GeneratedHTML,
		formatTimePtr(ct.SignedAt), ct.SignatureIP, ct.SignatureUserAgent, ct.PDFPath, formatTime(ct.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return db.GetContract(ct.ID)
}

func (db *DB) UpdateContractStatus(id string, status domain.ContractStatus, signedAt *time.Time, ip, userAgent string) error {
	_, err := db.Exec(
		`UPDATE contracts SET status=?, signed_at=?, signature_ip=?, signature_user_agent=? WHERE id=?`,
		status, formatTimePtr(signedAt), ip, userAgent, id,
	)
	return err
}

func (db *DB) GetContractForPatient(contractID, patientID string) (*domain.Contract, error) {
	row := db.QueryRow(`SELECT c.id, c.patient_id, p.name, c.template_id, ct.name,
		c.status, c.generated_html, c.signed_at, c.signature_ip, c.signature_user_agent, c.pdf_path, c.created_at
		FROM contracts c
		JOIN patients p ON p.id = c.patient_id
		JOIN contract_templates ct ON ct.id = c.template_id
		WHERE c.id = ? AND c.patient_id = ?`, contractID, patientID)
	return scanContract(row)
}

func scanContract(row *sql.Row) (*domain.Contract, error) {
	var c domain.Contract
	var signedAt sql.NullString
	var createdAt string
	err := row.Scan(&c.ID, &c.PatientID, &c.PatientName, &c.TemplateID, &c.TemplateName,
		&c.Status, &c.GeneratedHTML, &signedAt, &c.SignatureIP, &c.SignatureUserAgent, &c.PDFPath, &createdAt)
	if err != nil {
		return nil, err
	}
	c.SignedAt = parseTimePtr(signedAt)
	c.CreatedAt = parseTime(createdAt)
	return &c, nil
}

func scanContracts(rows *sql.Rows) ([]domain.Contract, error) {
	var list []domain.Contract
	for rows.Next() {
		var c domain.Contract
		var signedAt sql.NullString
		var createdAt string
		if err := rows.Scan(&c.ID, &c.PatientID, &c.PatientName, &c.TemplateID, &c.TemplateName,
			&c.Status, &c.GeneratedHTML, &signedAt, &c.SignatureIP, &c.SignatureUserAgent, &c.PDFPath, &createdAt); err != nil {
			return nil, err
		}
		c.SignedAt = parseTimePtr(signedAt)
		c.CreatedAt = parseTime(createdAt)
		list = append(list, c)
	}
	return list, rows.Err()
}
