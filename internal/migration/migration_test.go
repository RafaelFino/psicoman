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
	// Idempotência: o número aplicado é o número de migrations embutidas, e
	// não cresce na segunda execução.
	migs, err := load()
	if err != nil {
		t.Fatalf("carregando migrations embutidas: %v", err)
	}
	if count != len(migs) {
		t.Errorf("migrations aplicadas = %d, quer %d (total embutido)", count, len(migs))
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

// TestApprovalMigrationBackfill verifica a migration 0002 (mvp-audit1 R1):
// aplicada sobre uma base que já tinha pacientes, marca os pré-existentes como
// aprovado; novos registros ganham default pendente pela coluna.
func TestApprovalMigrationBackfill(t *testing.T) {
	// Base "legada": aplica só o schema inicial (0001) manualmente e insere um
	// paciente antes da migration de aprovação existir.
	db := openTemp(t)
	migs, err := load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := ensureTable(db.DB); err != nil {
		t.Fatalf("ensureTable: %v", err)
	}
	// Aplica apenas a 0001.
	for _, m := range migs {
		if m.Version == "0001" {
			if err := apply(db.DB, m); err != nil {
				t.Fatalf("aplicando 0001: %v", err)
			}
		}
	}
	// Paciente pré-existente (sem a coluna approval_status ainda).
	_, err = db.Exec(`INSERT INTO patient (id, name, phone, email, created_at, updated_at)
		VALUES ('01LEGADO','Legado','1199','legado@example.com','2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00')`)
	if err != nil {
		t.Fatalf("inserindo paciente legado: %v", err)
	}

	// Agora roda o Run completo (aplica as pendentes, incluindo a 0002).
	if err := Run(db.DB); err != nil {
		t.Fatalf("Run com migration de aprovação: %v", err)
	}

	// O pré-existente deve ter virado aprovado.
	var status string
	if err := db.QueryRow(`SELECT approval_status FROM patient WHERE id='01LEGADO'`).Scan(&status); err != nil {
		t.Fatalf("lendo approval_status: %v", err)
	}
	if status != "aprovado" {
		t.Errorf("paciente pré-existente ficou %q, quer aprovado", status)
	}

	// Novo registro sem informar a coluna cai no default pendente.
	_, err = db.Exec(`INSERT INTO patient (id, name, phone, email, created_at, updated_at)
		VALUES ('01NOVO','Novo','1188','novo@example.com','2026-02-01T09:00:00-03:00','2026-02-01T09:00:00-03:00')`)
	if err != nil {
		t.Fatalf("inserindo paciente novo: %v", err)
	}
	if err := db.QueryRow(`SELECT approval_status FROM patient WHERE id='01NOVO'`).Scan(&status); err != nil {
		t.Fatalf("lendo approval_status novo: %v", err)
	}
	if status != "pendente" {
		t.Errorf("novo registro ficou %q, quer pendente (default)", status)
	}
}

// TestPlanDueDayMigration verifica a migration 0003 (patient-timeline R1.5):
// aplicada sobre uma base com planos pré-existentes, os fechados ganham
// due_day = 5 e os demais ficam no default 1; novos registros caem no default.
func TestPlanDueDayMigration(t *testing.T) {
	db := openTemp(t)
	migs, err := load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := ensureTable(db.DB); err != nil {
		t.Fatalf("ensureTable: %v", err)
	}
	// Aplica 0001 e 0002 (base legada, antes da coluna due_day existir).
	for _, m := range migs {
		if m.Version == "0001" || m.Version == "0002" {
			if err := apply(db.DB, m); err != nil {
				t.Fatalf("aplicando %s: %v", m.Version, err)
			}
		}
	}
	// Paciente + planos pré-existentes (sem due_day ainda).
	_, err = db.Exec(`INSERT INTO patient (id, name, phone, email, approval_status, created_at, updated_at)
		VALUES ('01PAC','Pac','1199','pac@example.com','aprovado','2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00')`)
	if err != nil {
		t.Fatalf("inserindo paciente: %v", err)
	}
	_, err = db.Exec(`INSERT INTO plan (id, patient_id, type, amount, starts_at, created_at, updated_at)
		VALUES ('01FEC','01PAC','plano_fechado_mensal',50000,'2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00'),
		       ('01SES','01PAC','pagamento_por_consulta',0,'2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00','2026-01-01T09:00:00-03:00')`)
	if err != nil {
		t.Fatalf("inserindo planos: %v", err)
	}

	// Roda o Run completo (aplica 0003).
	if err := Run(db.DB); err != nil {
		t.Fatalf("Run com migration due_day: %v", err)
	}

	// Fechado pré-existente ⇒ due_day 5; por-sessão ⇒ default 1.
	var dueFechado, dueSessao int
	if err := db.QueryRow(`SELECT due_day FROM plan WHERE id='01FEC'`).Scan(&dueFechado); err != nil {
		t.Fatalf("lendo due_day fechado: %v", err)
	}
	if dueFechado != 5 {
		t.Errorf("plano fechado ficou due_day=%d, quer 5", dueFechado)
	}
	if err := db.QueryRow(`SELECT due_day FROM plan WHERE id='01SES'`).Scan(&dueSessao); err != nil {
		t.Fatalf("lendo due_day sessão: %v", err)
	}
	if dueSessao != 1 {
		t.Errorf("plano por sessão ficou due_day=%d, quer 1 (default)", dueSessao)
	}

	// Novo registro sem informar a coluna cai no default 1.
	_, err = db.Exec(`INSERT INTO plan (id, patient_id, type, amount, starts_at, created_at, updated_at)
		VALUES ('01NOVO','01PAC','plano_fechado_mensal',60000,'2026-02-01T09:00:00-03:00','2026-02-01T09:00:00-03:00','2026-02-01T09:00:00-03:00')`)
	if err != nil {
		t.Fatalf("inserindo plano novo: %v", err)
	}
	var dueNovo int
	if err := db.QueryRow(`SELECT due_day FROM plan WHERE id='01NOVO'`).Scan(&dueNovo); err != nil {
		t.Fatalf("lendo due_day novo: %v", err)
	}
	if dueNovo != 1 {
		t.Errorf("novo plano ficou due_day=%d, quer 1 (default)", dueNovo)
	}
}
