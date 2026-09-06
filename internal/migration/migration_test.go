package migration

import (
	"path/filepath"
	"testing"

	"github.com/RafaelFino/psicoman/internal/repository/sqlite"
)

func openTemp(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(sqlite.Options{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("abrindo banco: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunAppliesSchema(t *testing.T) {
	db := openTemp(t)
	if err := Run(db.DB); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Tabelas-chave devem existir.
	for _, table := range []string{
		"patient", "location", "session", "debt", "payment",
		"plan", "ged_file", "therapist_profile", "audit_log", "schema_migration",
	} {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("tabela %q não criada: %v", table, err)
		}
	}
}

func TestRunIsIdempotent(t *testing.T) {
	db := openTemp(t)
	if err := Run(db.DB); err != nil {
		t.Fatalf("primeira Run: %v", err)
	}
	// Segunda execução não deve reaplicar nem falhar.
	if err := Run(db.DB); err != nil {
		t.Fatalf("segunda Run: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migration`).Scan(&count); err != nil {
		t.Fatalf("contando migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("migrations aplicadas = %d, quer 1", count)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db := openTemp(t)
	if err := Run(db.DB); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Inserir sessão com patient_id inexistente deve violar FK.
	_, err := db.Exec(`INSERT INTO session
		(id, patient_id, modality, starts_at, ends_at, status, created_at, updated_at)
		VALUES ('01AAA','inexistente','online','2026-01-01T10:00:00-03:00','2026-01-01T11:00:00-03:00','solicitada','2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00')`)
	if err == nil {
		t.Fatal("esperava erro de foreign key, inserção passou")
	}
}
