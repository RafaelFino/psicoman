package storage

import (
	"time"

	"github.com/fino/psicoman/internal/domain"
)

func (db *DB) UpsertStaffUser(email string, role domain.Role) (*domain.StaffUser, error) {
	existing, err := db.GetStaffByEmail(email)
	if err == nil {
		return existing, nil
	}

	u := domain.StaffUser{
		ID:        email,
		Email:     email,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	_, err = db.Exec(
		`INSERT INTO staff_users (id, email, role, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, u.Email, u.Role, formatTime(u.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetStaffByEmail(email string) (*domain.StaffUser, error) {
	row := db.QueryRow(`SELECT id, email, role, created_at FROM staff_users WHERE email = ?`, email)
	var u domain.StaffUser
	var createdAt string
	if err := row.Scan(&u.ID, &u.Email, &u.Role, &createdAt); err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(createdAt)
	return &u, nil
}

func (db *DB) ListStaff() ([]domain.StaffUser, error) {
	rows, err := db.Query(`SELECT id, email, role, created_at FROM staff_users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.StaffUser
	for rows.Next() {
		var u domain.StaffUser
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &createdAt); err != nil {
			return nil, err
		}
		u.CreatedAt = parseTime(createdAt)
		list = append(list, u)
	}
	return list, rows.Err()
}
