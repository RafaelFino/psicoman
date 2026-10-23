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

func TestMigracaoIdempotente(t *testing.T) {
	app := newTestApp(t)

	antes := make(map[string]string, len(grupoA))
	for _, name := range grupoA {
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

	for _, name := range grupoA {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("coleção %q ausente após a 2ª execução: %v", name, err)
		}
		if col.Id != antes[name] {
			t.Errorf("coleção %q foi recriada (id mudou de %q para %q)", name, antes[name], col.Id)
		}
	}
}
