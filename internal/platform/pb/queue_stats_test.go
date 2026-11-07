package pb

import (
	"context"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

func TestQueueStats_ContaPendentes(t *testing.T) {
	a := newTestApp(t)
	q := NewQueueStats(a)
	ctx := context.Background()

	if n, err := q.PendingPatients(ctx); err != nil || n != 0 {
		t.Fatalf("banco vazio: quero 0 pacientes pendentes, veio %d err=%v", n, err)
	}
	if n, err := q.PendingAppointments(ctx); err != nil || n != 0 {
		t.Fatalf("banco vazio: quero 0 pedidos pendentes, veio %d err=%v", n, err)
	}

	col, err := a.FindCollectionByNameOrId("patient")
	if err != nil {
		t.Fatal(err)
	}
	novo := func(email, status string) {
		rec := core.NewRecord(col)
		rec.Set("id", ulid.New())
		rec.Set("name", "Fulano")
		rec.Set("email", email)
		rec.Set("approval_status", status)
		if err := a.Save(rec); err != nil {
			t.Fatalf("salvar paciente: %v", err)
		}
	}
	novo("a@x.com", "pendente")
	novo("b@x.com", "pendente")
	novo("c@x.com", "aprovado")
	novo("d@x.com", "rejeitado")

	if n, err := q.PendingPatients(ctx); err != nil || n != 2 {
		t.Errorf("quero 2 pacientes pendentes (aprovado/rejeitado não contam), veio %d err=%v", n, err)
	}
}
