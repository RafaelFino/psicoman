package storage

import (
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

// ─── Therapy Spaces ──────────────────────────────────────────────────────────

func (db *DB) ListSpaces() ([]domain.TherapySpace, error) {
	rows, err := db.Query(`SELECT id, name, address, type, cost_cents_per_use, cost_cents_monthly, is_available, notes, created_at FROM therapy_spaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.TherapySpace
	for rows.Next() {
		var s domain.TherapySpace
		var isAvailable int
		var createdAt string
		if err := rows.Scan(&s.ID, &s.Name, &s.Address, &s.Type, &s.CostCentsPerUse, &s.CostCentsMonthly, &isAvailable, &s.Notes, &createdAt); err != nil {
			return nil, err
		}
		s.IsAvailable = isAvailable == 1
		s.CreatedAt = parseTime(createdAt)
		list = append(list, s)
	}
	return list, rows.Err()
}

func (db *DB) GetSpace(id string) (*domain.TherapySpace, error) {
	row := db.QueryRow(`SELECT id, name, address, type, cost_cents_per_use, cost_cents_monthly, is_available, notes, created_at FROM therapy_spaces WHERE id = ?`, id)
	var s domain.TherapySpace
	var isAvailable int
	var createdAt string
	err := row.Scan(&s.ID, &s.Name, &s.Address, &s.Type, &s.CostCentsPerUse, &s.CostCentsMonthly, &isAvailable, &s.Notes, &createdAt)
	if err != nil {
		return nil, err
	}
	s.IsAvailable = isAvailable == 1
	s.CreatedAt = parseTime(createdAt)
	return &s, nil
}

func (db *DB) CreateSpace(s domain.TherapySpace) (*domain.TherapySpace, error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO therapy_spaces (id, name, address, type, cost_cents_per_use, cost_cents_monthly, is_available, notes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Address, s.Type, s.CostCentsPerUse, s.CostCentsMonthly, boolInt(s.IsAvailable), s.Notes, formatTime(s.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) UpdateSpace(s domain.TherapySpace) error {
	_, err := db.Exec(
		`UPDATE therapy_spaces SET name=?, address=?, type=?, cost_cents_per_use=?, cost_cents_monthly=?, is_available=?, notes=? WHERE id=?`,
		s.Name, s.Address, s.Type, s.CostCentsPerUse, s.CostCentsMonthly, boolInt(s.IsAvailable), s.Notes, s.ID,
	)
	return err
}

func (db *DB) DeleteSpace(id string) error {
	_, err := db.Exec(`DELETE FROM therapy_spaces WHERE id=?`, id)
	return err
}

// ─── Space Bookings ──────────────────────────────────────────────────────────

func (db *DB) ListSpaceBookings(spaceID, date string) ([]domain.SpaceBooking, error) {
	q := `SELECT sb.id, sb.space_id, ts.name, sb.appointment_id, sb.booking_date, sb.start_time, sb.end_time, sb.created_at
		FROM space_bookings sb JOIN therapy_spaces ts ON ts.id = sb.space_id WHERE 1=1`
	args := []any{}
	if spaceID != "" {
		q += ` AND sb.space_id = ?`
		args = append(args, spaceID)
	}
	if date != "" {
		q += ` AND sb.booking_date = ?`
		args = append(args, date)
	}
	q += ` ORDER BY sb.booking_date, sb.start_time`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.SpaceBooking
	for rows.Next() {
		var b domain.SpaceBooking
		var createdAt string
		if err := rows.Scan(&b.ID, &b.SpaceID, &b.SpaceName, &b.AppointmentID, &b.BookingDate, &b.StartTime, &b.EndTime, &createdAt); err != nil {
			return nil, err
		}
		b.CreatedAt = parseTime(createdAt)
		list = append(list, b)
	}
	return list, rows.Err()
}

func (db *DB) CreateSpaceBooking(b domain.SpaceBooking) (*domain.SpaceBooking, error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	b.CreatedAt = time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO space_bookings (id, space_id, appointment_id, booking_date, start_time, end_time, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.SpaceID, b.AppointmentID, b.BookingDate, b.StartTime, b.EndTime, formatTime(b.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (db *DB) DeleteSpaceBooking(id string) error {
	_, err := db.Exec(`DELETE FROM space_bookings WHERE id=?`, id)
	return err
}
