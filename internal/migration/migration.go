// Package migration aplica migrations SQL versionadas no boot.
//
// Convenções (psicoman-sqlite.md): migrations append-only, numeradas e
// versionadas em migrations/*.sql, aplicadas em ordem, registradas em
// schema_migration. Nunca editar uma migration já aplicada.
package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

//go:embed sql/*.sql
var migrationsFS embed.FS

// Migration é um arquivo de migration carregado.
type Migration struct {
	Version string // ex: "0001"
	Name    string // ex: "schema_inicial"
	SQL     string
}

// Run aplica todas as migrations pendentes em ordem crescente de versão.
func Run(db *sql.DB) error {
	if err := ensureTable(db); err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	migs, err := load()
	if err != nil {
		return err
	}
	for _, m := range migs {
		if applied[m.Version] {
			continue
		}
		if err := apply(db, m); err != nil {
			return fmt.Errorf("migration %s (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func ensureTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migration (
		version    TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("criando schema_migration: %w", err)
	}
	return nil
}

func appliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migration`)
	if err != nil {
		return nil, fmt.Errorf("lendo migrations aplicadas: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func apply(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.SQL); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migration (version, name, applied_at) VALUES (?, ?, ?)`,
		m.Version, m.Name, clock.Format(clock.Now()),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// load lê e ordena as migrations embutidas.
func load() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("lendo migrations embutidas: %w", err)
	}
	var migs []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		content, err := migrationsFS.ReadFile("sql/" + e.Name())
		if err != nil {
			return nil, err
		}
		version, name := parseName(e.Name())
		migs = append(migs, Migration{Version: version, Name: name, SQL: string(content)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return migs, nil
}

// parseName extrai versão e nome de "0001_schema_inicial.sql".
func parseName(filename string) (version, name string) {
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return base, base
}
