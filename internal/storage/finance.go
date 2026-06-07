package storage

import (
	"database/sql"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/google/uuid"
)

func (db *DB) ListPayments(month, year int) ([]domain.Payment, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rows, err := db.Query(
		`SELECT pay.id, pay.patient_id, p.name, pay.appointment_id, pay.amount_cents, pay.status, pay.due_date, pay.received_at, pay.invoice_number
		FROM payments pay JOIN patients p ON p.id = pay.patient_id
		WHERE pay.due_date >= ? AND pay.due_date < ? ORDER BY pay.due_date`,
		formatTime(start), formatTime(end),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

func (db *DB) CreatePayment(p domain.Payment) (*domain.Payment, error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	_, err := db.Exec(
		`INSERT INTO payments (id, patient_id, appointment_id, amount_cents, status, due_date, received_at, invoice_number) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.PatientID, p.AppointmentID, p.AmountCents, p.Status, formatTime(p.DueDate), formatTimePtr(p.ReceivedAt), p.InvoiceNumber,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) ReceivePayment(id string) error {
	_, err := db.Exec(
		`UPDATE payments SET status=?, received_at=? WHERE id=?`,
		domain.PaymentReceived, formatTime(time.Now().UTC()), id,
	)
	return err
}

func (db *DB) ListCosts(month, year int) ([]domain.Cost, error) {
	rows, err := db.Query(
		`SELECT id, description, amount_cents, month, year, category FROM costs WHERE month=? AND year=? ORDER BY description`,
		month, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Cost
	for rows.Next() {
		var c domain.Cost
		if err := rows.Scan(&c.ID, &c.Description, &c.AmountCents, &c.Month, &c.Year, &c.Category); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (db *DB) CreateCost(c domain.Cost) (*domain.Cost, error) {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	_, err := db.Exec(
		`INSERT INTO costs (id, description, amount_cents, month, year, category) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Description, c.AmountCents, c.Month, c.Year, c.Category,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func scanPayments(rows *sql.Rows) ([]domain.Payment, error) {
	var list []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var dueDate, receivedAt sql.NullString
		if err := rows.Scan(&p.ID, &p.PatientID, &p.PatientName, &p.AppointmentID, &p.AmountCents, &p.Status, &dueDate, &receivedAt, &p.InvoiceNumber); err != nil {
			return nil, err
		}
		p.DueDate = parseTime(dueDate.String)
		p.ReceivedAt = parseTimePtr(receivedAt)
		list = append(list, p)
	}
	return list, rows.Err()
}
