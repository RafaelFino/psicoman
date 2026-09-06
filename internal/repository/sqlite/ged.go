package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// GEDMetaRepo implementa service.GEDMetaRepository sobre SQLite.
type GEDMetaRepo struct{ db *sql.DB }

// NewGEDMetaRepo cria o repositório de metadados do GED.
func NewGEDMetaRepo(db *DB) *GEDMetaRepo { return &GEDMetaRepo{db: db.DB} }

var _ service.GEDMetaRepository = (*GEDMetaRepo)(nil)

// Insert grava o metadado de um arquivo.
func (r *GEDMetaRepo) Insert(ctx context.Context, f *domain.GEDFile) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ged_file (id, patient_id, session_id, debt_id, payment_id, rel_path, mime, size, sha256, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, nullStr(f.PatientID), nullStr(f.SessionID), nullStr(f.DebtID), nullStr(f.PaymentID),
		f.RelPath, nullStr(f.MIME), f.Size, f.SHA256, clock.Format(f.CreatedAt))
	return mapError(err)
}

// Get busca um metadado por id.
func (r *GEDMetaRepo) Get(ctx context.Context, id string) (*domain.GEDFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, patient_id, session_id, debt_id, payment_id, rel_path, mime, size, sha256, created_at
		 FROM ged_file WHERE id=?`, id)
	return scanGED(row)
}

// FindByHash busca dedup por hash dentro do escopo do paciente.
// patientID vazio corresponde a arquivos do perfil do terapeuta (patient_id IS NULL).
func (r *GEDMetaRepo) FindByHash(ctx context.Context, patientID, sha string) (*domain.GEDFile, error) {
	var row *sql.Row
	if patientID == "" {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, patient_id, session_id, debt_id, payment_id, rel_path, mime, size, sha256, created_at
			 FROM ged_file WHERE patient_id IS NULL AND sha256=?`, sha)
	} else {
		row = r.db.QueryRowContext(ctx,
			`SELECT id, patient_id, session_id, debt_id, payment_id, rel_path, mime, size, sha256, created_at
			 FROM ged_file WHERE patient_id=? AND sha256=?`, patientID, sha)
	}
	f, err := scanGED(row)
	if err == service.ErrNotFound {
		return nil, service.ErrNotFound
	}
	return f, err
}

// ListByPatient devolve os arquivos de um paciente (mais recentes primeiro).
func (r *GEDMetaRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.GEDFile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, patient_id, session_id, debt_id, payment_id, rel_path, mime, size, sha256, created_at
		 FROM ged_file WHERE patient_id=? ORDER BY created_at DESC`, patientID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.GEDFile
	for rows.Next() {
		f, err := scanGEDFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Delete remove o metadado.
func (r *GEDMetaRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ged_file WHERE id=?`, id)
	return mapError(err)
}

// AllFiles devolve todas as referências (rel_path + sha256) para backup.
func (r *GEDMetaRepo) AllFiles(ctx context.Context) ([]service.GEDFileRef, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT rel_path, sha256 FROM ged_file`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []service.GEDFileRef
	for rows.Next() {
		var ref service.GEDFileRef
		if err := rows.Scan(&ref.RelPath, &ref.SHA256); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

func scanGED(row *sql.Row) (*domain.GEDFile, error) {
	f, err := scanGEDFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return f, nil
}

func scanGEDFrom(s scanner) (*domain.GEDFile, error) {
	var (
		f                                       domain.GEDFile
		patientID, sessionID, debtID, paymentID sql.NullString
		mime                                    sql.NullString
		createdAt                               string
	)
	if err := s.Scan(&f.ID, &patientID, &sessionID, &debtID, &paymentID,
		&f.RelPath, &mime, &f.Size, &f.SHA256, &createdAt); err != nil {
		return nil, err
	}
	f.PatientID = patientID.String
	f.SessionID = sessionID.String
	f.DebtID = debtID.String
	f.PaymentID = paymentID.String
	f.MIME = mime.String
	if t, err := clock.Parse(createdAt); err == nil {
		f.CreatedAt = t
	}
	return &f, nil
}
