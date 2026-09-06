package sqlite

import (
	"context"
	"database/sql"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/service"
)

// AppointmentRepo implementa service.AppointmentRepository sobre SQLite.
type AppointmentRepo struct{ db *sql.DB }

// NewAppointmentRepo cria o repositório de pedidos de agendamento.
func NewAppointmentRepo(db *DB) *AppointmentRepo { return &AppointmentRepo{db: db.DB} }

var _ service.AppointmentRepository = (*AppointmentRepo)(nil)

const apptSelect = `SELECT id, patient_id, location_id, slot_start, slot_end, status, note, created_at, updated_at FROM appointment_request`

// Create insere um pedido.
func (r *AppointmentRepo) Create(ctx context.Context, a *domain.AppointmentRequest) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO appointment_request (id, patient_id, location_id, slot_start, slot_end, status, note, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.PatientID, nullStr(a.LocationID), clock.Format(a.SlotStart), clock.Format(a.SlotEnd),
		a.Status, nullStr(a.Note), clock.Format(a.CreatedAt), clock.Format(a.UpdatedAt))
	return mapError(err)
}

// Get busca um pedido por id.
func (r *AppointmentRepo) Get(ctx context.Context, id string) (*domain.AppointmentRequest, error) {
	row := r.db.QueryRowContext(ctx, apptSelect+` WHERE id=?`, id)
	return scanAppt(row)
}

// ListPending devolve os pedidos pendentes (mais antigos primeiro).
func (r *AppointmentRepo) ListPending(ctx context.Context) ([]*domain.AppointmentRequest, error) {
	return r.query(ctx, apptSelect+` WHERE status='pendente' ORDER BY slot_start`)
}

// ListByPatient devolve os pedidos de um paciente.
func (r *AppointmentRepo) ListByPatient(ctx context.Context, patientID string) ([]*domain.AppointmentRequest, error) {
	return r.query(ctx, apptSelect+` WHERE patient_id=? ORDER BY created_at DESC`, patientID)
}

// UpdateStatus atualiza o status de um pedido.
func (r *AppointmentRepo) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE appointment_request SET status=?, updated_at=? WHERE id=?`,
		status, clock.Format(clock.Now()), id)
	if err != nil {
		return mapError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrNotFound
	}
	return nil
}

func (r *AppointmentRepo) query(ctx context.Context, q string, args ...any) ([]*domain.AppointmentRequest, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var out []*domain.AppointmentRequest
	for rows.Next() {
		a, err := scanApptFrom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanAppt(row *sql.Row) (*domain.AppointmentRequest, error) {
	a, err := scanApptFrom(row)
	if err != nil {
		return nil, mapError(err)
	}
	return a, nil
}

func scanApptFrom(sc scanner) (*domain.AppointmentRequest, error) {
	var (
		a                  domain.AppointmentRequest
		locationID, note   sql.NullString
		slotStart, slotEnd string
		createdAt, updated string
	)
	if err := sc.Scan(&a.ID, &a.PatientID, &locationID, &slotStart, &slotEnd,
		&a.Status, &note, &createdAt, &updated); err != nil {
		return nil, err
	}
	a.LocationID = locationID.String
	a.Note = note.String
	if t, err := clock.Parse(slotStart); err == nil {
		a.SlotStart = t
	}
	if t, err := clock.Parse(slotEnd); err == nil {
		a.SlotEnd = t
	}
	if t, err := clock.Parse(createdAt); err == nil {
		a.CreatedAt = t
	}
	if t, err := clock.Parse(updated); err == nil {
		a.UpdatedAt = t
	}
	return &a, nil
}
