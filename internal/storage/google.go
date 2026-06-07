package storage

import "time"

type GoogleTokens struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

func (db *DB) SaveGoogleTokens(t GoogleTokens) error {
	_, err := db.Exec(
		`INSERT INTO google_tokens (id, access_token, refresh_token, expiry) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET access_token=excluded.access_token, refresh_token=excluded.refresh_token, expiry=excluded.expiry`,
		t.AccessToken, t.RefreshToken, formatTime(t.Expiry),
	)
	return err
}

func (db *DB) GetGoogleTokens() (*GoogleTokens, error) {
	row := db.QueryRow(`SELECT access_token, refresh_token, expiry FROM google_tokens WHERE id=1`)
	var t GoogleTokens
	var expiry string
	if err := row.Scan(&t.AccessToken, &t.RefreshToken, &expiry); err != nil {
		return nil, err
	}
	t.Expiry = parseTime(expiry)
	return &t, nil
}

func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	return val, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}
