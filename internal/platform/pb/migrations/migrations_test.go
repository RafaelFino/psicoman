package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// newTestApp sobe um PocketBase embarcado sobre um DataDir temporário e aplica
// todas as migrations (inclusive as deste pacote, registradas via init()).
// Replica o que tests.NewTestApp faz internamente, sem depender do dir-template.
func newTestApp(t *testing.T) core.App {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatalf("RunAllMigrations: %v", err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	return app
}

// grupoA são as coleções criadas pela migration 0010.
var grupoA = []string{
	"origin",
	"location",
	"patient",
	"availability",
	"appointment_request",
	"session",
}

func TestGrupoAColecoesExistem(t *testing.T) {
	app := newTestApp(t)
	for _, name := range grupoA {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Errorf("coleção %q deveria existir após a migração: %v", name, err)
		}
	}
}

func TestIdULIDCustomAceitoEPreservado(t *testing.T) {
	app := newTestApp(t)
	col, err := app.FindCollectionByNameOrId("origin")
	if err != nil {
		t.Fatal(err)
	}
	id := ulid.New()
	rec := core.NewRecord(col)
	rec.Set("id", id)
	rec.Set("name", "Indicação")
	if err := app.Save(rec); err != nil {
		t.Fatalf("Save com id ULID deveria funcionar: %v", err)
	}
	got, err := app.FindRecordById(col, id)
	if err != nil {
		t.Fatalf("FindRecordById: %v", err)
	}
	if got.Id != id {
		t.Errorf("id não preservado: quero %q, veio %q", id, got.Id)
	}
}

func TestPatientUniqueEmail(t *testing.T) {
	app := newTestApp(t)
	col, err := app.FindCollectionByNameOrId("patient")
	if err != nil {
		t.Fatal(err)
	}

	novo := func(email, cpf string) *core.Record {
		rec := core.NewRecord(col)
		rec.Set("id", ulid.New())
		rec.Set("name", "Fulano")
		rec.Set("phone", "11999999999")
		rec.Set("email", email)
		rec.Set("cpf", cpf)
		rec.Set("approval_status", "pendente")
		return rec
	}

	if err := app.Save(novo("a@exemplo.com", "")); err != nil {
		t.Fatalf("1º patient deveria salvar: %v", err)
	}
	if err := app.Save(novo("a@exemplo.com", "")); err == nil {
		t.Error("2º patient com mesmo email deveria falhar (UNIQUE ux_patient_email)")
	}
}

func TestPatientUniqueCPFParcial(t *testing.T) {
	app := newTestApp(t)
	col, err := app.FindCollectionByNameOrId("patient")
	if err != nil {
		t.Fatal(err)
	}

	novo := func(email, cpf string) *core.Record {
		rec := core.NewRecord(col)
		rec.Set("id", ulid.New())
		rec.Set("name", "Fulano")
		rec.Set("email", email)
		rec.Set("cpf", cpf)
		rec.Set("approval_status", "pendente")
		return rec
	}

	// mesmo CPF não-nulo repetido deve falhar.
	if err := app.Save(novo("b1@exemplo.com", "12345678901")); err != nil {
		t.Fatalf("patient com cpf deveria salvar: %v", err)
	}
	if err := app.Save(novo("b2@exemplo.com", "12345678901")); err == nil {
		t.Error("2º patient com mesmo cpf deveria falhar (UNIQUE ux_patient_cpf)")
	}

	// cpf vazio repetido NÃO deve falhar (índice parcial).
	if err := app.Save(novo("c1@exemplo.com", "")); err != nil {
		t.Fatalf("patient com cpf vazio deveria salvar: %v", err)
	}
	if err := app.Save(novo("c2@exemplo.com", "")); err != nil {
		t.Errorf("2º patient com cpf vazio NÃO deveria falhar (índice parcial): %v", err)
	}
}

// grupoB são as coleções criadas pela migration 0020.
var grupoB = []string{
	"cost_category",
	"cost_item",
	"contract",
	"plan",
	"debt",
	"payment",
	"session_cost",
}

func TestGrupoBColecoesExistem(t *testing.T) {
	app := newTestApp(t)
	for _, name := range grupoB {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Errorf("coleção %q deveria existir após a migração: %v", name, err)
		}
	}
}

// TestRelationsDeCicloExistem confere que os patches de relation (ciclos
// contract<->plan e session->contract/plan) adicionaram os campos esperados.
func TestRelationsDeCicloExistem(t *testing.T) {
	app := newTestApp(t)

	casos := []struct {
		collection string
		field      string
	}{
		{"contract", "plan_id"},
		{"contract", "patient_id"},
		{"plan", "contract_id"},
		{"session", "contract_id"},
		{"session", "plan_id"},
	}
	for _, c := range casos {
		col, err := app.FindCollectionByNameOrId(c.collection)
		if err != nil {
			t.Fatalf("coleção %q ausente: %v", c.collection, err)
		}
		f := col.Fields.GetByName(c.field)
		if f == nil {
			t.Errorf("%s.%s deveria existir", c.collection, c.field)
			continue
		}
		if _, ok := f.(*core.RelationField); !ok {
			t.Errorf("%s.%s deveria ser RelationField, veio %T", c.collection, c.field, f)
		}
	}
}

func TestDebtUniqueIdempotencyKey(t *testing.T) {
	app := newTestApp(t)
	patientID := criaPatient(t, app, "debt@exemplo.com")

	debtCol, err := app.FindCollectionByNameOrId("debt")
	if err != nil {
		t.Fatal(err)
	}
	novo := func() *core.Record {
		rec := core.NewRecord(debtCol)
		rec.Set("id", ulid.New())
		rec.Set("patient_id", patientID)
		rec.Set("amount", 15000)
		rec.Set("status", "aberto")
		rec.Set("idempotency_key", "sessao-abc-123")
		return rec
	}

	if err := app.Save(novo()); err != nil {
		t.Fatalf("1ª debt deveria salvar: %v", err)
	}
	if err := app.Save(novo()); err == nil {
		t.Error("2ª debt com mesma idempotency_key deveria falhar (UNIQUE ux_debt_idempotency)")
	}
}

func TestSessionCostUniqueSessionID(t *testing.T) {
	app := newTestApp(t)
	patientID := criaPatient(t, app, "scost@exemplo.com")
	sessionID := criaSession(t, app, patientID)

	scCol, err := app.FindCollectionByNameOrId("session_cost")
	if err != nil {
		t.Fatal(err)
	}
	novo := func() *core.Record {
		rec := core.NewRecord(scCol)
		rec.Set("id", ulid.New())
		rec.Set("session_id", sessionID)
		rec.Set("amount", 3000)
		rec.Set("method", "direto")
		return rec
	}

	if err := app.Save(novo()); err != nil {
		t.Fatalf("1º session_cost deveria salvar: %v", err)
	}
	if err := app.Save(novo()); err == nil {
		t.Error("2º session_cost com mesmo session_id deveria falhar (UNIQUE ux_session_cost_session)")
	}
}

// criaPatient insere um patient mínimo e devolve seu id.
func criaPatient(t *testing.T, app core.App, email string) string {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("patient")
	if err != nil {
		t.Fatal(err)
	}
	id := ulid.New()
	rec := core.NewRecord(col)
	rec.Set("id", id)
	rec.Set("name", "Fulano")
	rec.Set("email", email)
	rec.Set("approval_status", "aprovado")
	if err := app.Save(rec); err != nil {
		t.Fatalf("cria patient: %v", err)
	}
	return id
}

// criaSession insere uma session mínima para o paciente e devolve seu id.
func criaSession(t *testing.T, app core.App, patientID string) string {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("session")
	if err != nil {
		t.Fatal(err)
	}
	id := ulid.New()
	rec := core.NewRecord(col)
	rec.Set("id", id)
	rec.Set("patient_id", patientID)
	rec.Set("status", "agendada")
	if err := app.Save(rec); err != nil {
		t.Fatalf("cria session: %v", err)
	}
	return id
}

func TestMigracaoIdempotente(t *testing.T) {
	app := newTestApp(t)

	todas := append(append([]string{}, grupoA...), grupoB...)
	antes := make(map[string]string, len(todas))
	for _, name := range todas {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("coleção %q ausente antes da reexecução: %v", name, err)
		}
		antes[name] = col.Id
	}

	// rodar de novo não deve erRar nem recriar/duplicar coleções.
	if err := app.RunAllMigrations(); err != nil {
		t.Fatalf("RunAllMigrations 2x deveria ser no-op: %v", err)
	}

	for _, name := range todas {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("coleção %q ausente após a 2ª execução: %v", name, err)
		}
		if col.Id != antes[name] {
			t.Errorf("coleção %q foi recriada (id mudou de %q para %q)", name, antes[name], col.Id)
		}
	}
}
