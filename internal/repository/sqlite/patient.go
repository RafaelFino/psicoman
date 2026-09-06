package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// PatientRepo implementa service.PatientRepository sobre SQLite.
type PatientRepo struct{ db *sql.DB }

// NewPatientRepo cria o repositório de pacientes.
func NewPatientRepo(db *DB) *PatientRepo { return &PatientRepo{db: db.DB} }

var _ service.PatientRepository = (*PatientRepo)(nil)

// Create insere um paciente.
func (r *PatientRepo) Create(ctx context.Context, p *domain.Patient) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO patient (id, name, phone, email, cpf, origin_id, approval_status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Phone, p.Email, nullStr(p.CPF), nullStr(p.OriginID), p.ApprovalStatus,
		clock.Format(p.CreatedAt), clock.Format(p.UpdatedAt),
	)
	return mapError(err)
}

// Update atualiza um paciente.
func (r *PatientRepo) Update(ctx context.Context, p *domain.Patient) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE patient SET name=?, phone=?, email=?, cpf=?, origin_id=?, approval_status=?, updated_at=?
		 WHERE id=? AND deleted_at IS NULL`,
		p.Name, p.Phone, p.Email, nullStr(p.CPF), nullStr(p.OriginID), p.ApprovalStatus,
		clock.Format(p.UpdatedAt), p.ID,
	)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

// Get busca um paciente ativo por id.
func (r *PatientRepo) Get(ctx context.Context, id string) (*domain.Patient, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, phone, email, cpf, origin_id, approval_status, created_at, updated_at
		 FROM patient WHERE id=? AND deleted_at IS NULL`, id)
	return scanPatient(row)
}

// GetByEmail busca um paciente ativo por email.
func (r *PatientRepo) GetByEmail(ctx context.Context, email string) (*domain.Patient, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, phone, email, cpf, origin_id, approval_status, created_at, updated_at
		 FROM patient WHERE email=? AND deleted_at IS NULL`, email)
	return scanPatient(row)
}

// List devolve os pacientes ativos ordenados por criação.
func (r *PatientRepo) List(ctx context.Context) ([]*domain.Patient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, phone, email, cpf, origin_id, approval_status, created_at, updated_at
		 FROM patient WHERE deleted_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Patient
	for rows.Next() {
		p, err := scanPatientRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListByApproval devolve os pacientes ativos com o estado de aprovação
// informado, ordenados por criação (mais antigos primeiro, para a fila).
func (r *PatientRepo) ListByApproval(ctx context.Context, status string) ([]*domain.Patient, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, phone, email, cpf, origin_id, approval_status, created_at, updated_at
		 FROM patient WHERE approval_status=? AND deleted_at IS NULL ORDER BY created_at ASC`, status)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.Patient
	for rows.Next() {
		p, err := scanPatientRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SoftDelete marca o paciente como removido.
func (r *PatientRepo) SoftDelete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE patient SET deleted_at=? WHERE id=? AND deleted_at IS NULL`,
		clock.Format(clock.Now()), id)
	return mapError(err)
}

// EmailExists indica se há outro paciente ativo com o email.
func (r *PatientRepo) EmailExists(ctx context.Context, email, exceptID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM patient WHERE email=? AND id<>? AND deleted_at IS NULL`,
		email, exceptID).Scan(&count)
	return count > 0, mapError(err)
}

// CPFExists indica se há outro paciente ativo com o CPF.
func (r *PatientRepo) CPFExists(ctx context.Context, cpf, exceptID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM patient WHERE cpf=? AND id<>? AND deleted_at IS NULL`,
		cpf, exceptID).Scan(&count)
	return count > 0, mapError(err)
}

// scanner é a interface comum de *sql.Row e *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanPatient(row *sql.Row) (*domain.Patient, error) {
	p, err := scanPatientFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return p, nil
}

func scanPatientRows(rows *sql.Rows) (*domain.Patient, error) {
	return scanPatientFrom(rows)
}

func scanPatientFrom(s scanner) (*domain.Patient, error) {
	var (
		p                  domain.Patient
		cpf, originID      sql.NullString
		createdAt, updated string
	)
	if err := s.Scan(&p.ID, &p.Name, &p.Phone, &p.Email, &cpf, &originID, &p.ApprovalStatus, &createdAt, &updated); err != nil {
		return nil, err
	}
	p.CPF = cpf.String
	p.OriginID = originID.String
	if t, err := clock.Parse(createdAt); err == nil {
		p.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		p.UpdatedAt = t
	}
	return &p, nil
}
