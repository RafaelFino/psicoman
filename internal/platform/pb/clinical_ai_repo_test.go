package pb

import (
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// seedSessionPB insere uma sessão mínima via adapter e devolve seu id (para o FK
// de ai_summary.session_id e o ano do tag_index).
func seedSessionPB(t *testing.T, a core.App, patientID string, startsAt time.Time) string {
	t.Helper()
	sid := ulid.New()
	if err := NewSessionRepository(a).Create(ctx(), &scheduling.Session{
		ID: sid, PatientID: patientID, Modality: scheduling.ModalityOnline,
		Status: scheduling.SessionRealizada, StartsAt: startsAt, EndsAt: startsAt.Add(time.Hour),
		CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return sid
}

// TestAISummaryCifraNaBorda: ai_summary.resumo é clínico — persiste só o
// envelope cifrado (não-vazamento) e decifra no round-trip pelo adapter.
func TestAISummaryCifraNaBorda(t *testing.T) {
	a := newTestApp(t)
	cipher := testCipher(t)
	pid := seedPatient(t, a, "aisum@exemplo.com")
	sid := seedSessionPB(t, a, pid, time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()))
	repo := NewAISummaryRepository(a, cipher)

	const segredo = "resumo clínico sensível da sessão"
	s := &clinical.AISummary{
		ID: ulid.New(), SessionID: sid, Resumo: domain.NewSealedText(segredo),
		Status: clinical.AISummarySugerido, GeneratedAt: clock.Now(),
	}
	if err := repo.Create(ctx(), s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx(), s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Resumo.Reveal() != segredo {
		t.Errorf("claro não round-trip: %q", got.Resumo.Reveal())
	}
	// Não-vazamento: a coluna crua guarda só o envelope.
	raw, err := a.FindRecordById("ai_summary", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	env := raw.GetString("resumo")
	if strings.Contains(env, segredo) {
		t.Errorf("VAZAMENTO: coluna contém o claro: %q", env)
	}
	if !strings.HasPrefix(env, "v1.aes-256-gcm.") {
		t.Errorf("prefixo de envelope inesperado: %q", env)
	}

	// UpdateStatus muda o ciclo de revisão.
	if err := repo.UpdateStatus(ctx(), s.ID, clinical.AISummaryConsolidado); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx(), s.ID)
	if got.Status != clinical.AISummaryConsolidado {
		t.Errorf("status não atualizado: %q", got.Status)
	}
}

// TestTagIndexMergeEIsolamento: Upsert faz merge idempotente de session_ids e a
// busca é isolada por paciente (nunca cruza pacientes — R10.2).
func TestTagIndexMergeEIsolamento(t *testing.T) {
	a := newTestApp(t)
	p1 := seedPatient(t, a, "tag-p1@exemplo.com")
	p2 := seedPatient(t, a, "tag-p2@exemplo.com")
	repo := NewTagIndexRepository(a)

	// p1: duas sessões com a mesma tag no mesmo ano.
	if err := repo.Upsert(ctx(), p1, 2026, "ansiedade", "s1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(ctx(), p1, 2026, "ansiedade", "s2"); err != nil {
		t.Fatal(err)
	}
	// Reinserir s1 é idempotente (não duplica).
	if err := repo.Upsert(ctx(), p1, 2026, "ansiedade", "s1"); err != nil {
		t.Fatal(err)
	}
	ids, err := repo.FindByTag(ctx(), p1, 2026, "ansiedade")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("esperava 2 sessões (merge sem duplicata), veio %v", ids)
	}

	// p2 com a mesma tag: isolado — não enxerga as sessões de p1.
	if err := repo.Upsert(ctx(), p2, 2026, "ansiedade", "s9"); err != nil {
		t.Fatal(err)
	}
	ids2, err := repo.FindByTag(ctx(), p2, 2026, "ansiedade")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids2) != 1 || ids2[0] != "s9" {
		t.Fatalf("isolamento por paciente quebrado: %v", ids2)
	}
}
