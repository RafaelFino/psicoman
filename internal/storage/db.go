package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
	TenantID string
}

func Open(dataDir, tenantID string) (*DB, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(dbDir, tenantID+".sqlite")
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	db := &DB{DB: sqlDB, TenantID: tenantID}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	migrations := []string{
		"migrations/001_init.sql",
		"migrations/002_session_notes.sql",
		"migrations/003_anamnesis.sql",
		"migrations/004_contracts.sql",
		"migrations/005_supervisions.sql",
		"migrations/006_spaces.sql",
	}
	for _, name := range migrations {
		data, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("run migration %s: %w", name, err)
		}
	}
	return nil
}
