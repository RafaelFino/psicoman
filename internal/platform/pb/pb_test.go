package pb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/crypto"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"

	// Blank import para registrar as migrations (coleções do MER) no AppMigrations.
	_ "github.com/RafaelFino/psicoman/internal/platform/pb/migrations"
)

// newTestApp sobe um PocketBase embarcado sobre um DataDir temporário e aplica
// todas as migrations (coleções do MER). Mesmo padrão dos testes de migrations.
func newTestApp(t *testing.T) core.App {
	t.Helper()
	a := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := a.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if err := a.RunAllMigrations(); err != nil {
		t.Fatalf("RunAllMigrations: %v", err)
	}
	t.Cleanup(func() { _ = a.ResetBootstrapState() })
	return a
}

// testCipher devolve um domain.Cipher AES-256-GCM determinístico para os testes
// de cifra na borda (chave fixa de 32 bytes).
func testCipher(t *testing.T) domain.Cipher {
	t.Helper()
	const b64Key = "wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY="
	kp, err := crypto.NewMapKeyProvider("k1", []crypto.KeySpec{{ID: "k1", B64Key: b64Key}})
	if err != nil {
		t.Fatalf("NewMapKeyProvider: %v", err)
	}
	return crypto.New(kp)
}

func ctx() context.Context { return context.Background() }

// seedPatient insere um paciente mínimo via adapter e devolve seu id.
func seedPatient(t *testing.T, a core.App, email string) string {
	t.Helper()
	repo := NewPatientRepository(a)
	p := &patient.Patient{
		ID:             ulid.New(),
		Name:           "Fulano",
		Phone:          "5511999990001",
		Email:          email,
		ApprovalStatus: patient.PatientApproved,
		CreatedAt:      clock.Now(),
		UpdatedAt:      clock.Now(),
	}
	if err := repo.Create(ctx(), p); err != nil {
		t.Fatalf("seedPatient: %v", err)
	}
	return p.ID
}

// --- round-trip CRUD ---

func TestPatientRoundTrip(t *testing.T) {
	a := newTestApp(t)
	repo := NewPatientRepository(a)

	now := clock.Now()
	p := &patient.Patient{
		ID:             ulid.New(),
		Name:           "Maria",
		Phone:          "5511999990001",
		Email:          "maria@exemplo.com",
		CPF:            "39053344705",
		ApprovalStatus: patient.PatientPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.Create(ctx(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx(), p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p.ID || got.Name != "Maria" || got.Email != "maria@exemplo.com" || got.CPF != "39053344705" {
		t.Errorf("campos não preservados: %+v", got)
	}
	if clock.Format(got.CreatedAt) != clock.Format(now) {
		t.Errorf("created_at: quero %q, veio %q", clock.Format(now), clock.Format(got.CreatedAt))
	}

	// GetByEmail e List.
	byEmail, err := repo.GetByEmail(ctx(), "maria@exemplo.com")
	if err != nil || byEmail.ID != p.ID {
		t.Errorf("GetByEmail falhou: %v / %+v", err, byEmail)
	}
	list, err := repo.List(ctx())
	if err != nil || len(list) != 1 {
		t.Errorf("List: quero 1, veio %d (%v)", len(list), err)
	}

	// Update.
	got.Name = "Maria Silva"
	if err := repo.Update(ctx(), got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	reread, _ := repo.Get(ctx(), p.ID)
	if reread.Name != "Maria Silva" {
		t.Errorf("Update não persistiu: %q", reread.Name)
	}
}

func TestSessionRoundTripAndRange(t *testing.T) {
	a := newTestApp(t)
	pid := seedPatient(t, a, "sess@exemplo.com")
	repo := NewSessionRepository(a)

	start := time.Date(2026, 3, 10, 14, 0, 0, 0, clock.Location())
	s := &scheduling.Session{
		ID:        ulid.New(),
		PatientID: pid,
		Modality:  scheduling.ModalityOnline,
		StartsAt:  start,
		EndsAt:    start.Add(time.Hour),
		Status:    scheduling.SessionAgendada,
		CreatedAt: clock.Now(),
		UpdatedAt: clock.Now(),
	}
	if err := repo.Create(ctx(), s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx(), s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PatientID != pid || got.Status != scheduling.SessionAgendada {
		t.Errorf("campos não preservados: %+v", got)
	}
	if clock.Format(got.StartsAt) != clock.Format(start) {
		t.Errorf("starts_at: quero %q, veio %q", clock.Format(start), clock.Format(got.StartsAt))
	}

	// ListByRange: dentro e fora da janela.
	in, err := repo.ListByRange(ctx(), start.Add(-time.Hour), start.Add(time.Hour))
	if err != nil || len(in) != 1 {
		t.Errorf("ListByRange dentro: quero 1, veio %d (%v)", len(in), err)
	}
	out, err := repo.ListByRange(ctx(), start.Add(48*time.Hour), start.Add(72*time.Hour))
	if err != nil || len(out) != 0 {
		t.Errorf("ListByRange fora: quero 0, veio %d (%v)", len(out), err)
	}

	// CountRealizadasNoLocalNoPeriodo (precisa de um local real: relation).
	locRepo := NewLocationRepository(a)
	loc := &scheduling.Location{
		ID: ulid.New(), Name: "Consultório", Modality: scheduling.ModalityPresencial,
		CostPeriod: scheduling.PeriodMensal, CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
	}
	if err := locRepo.Create(ctx(), loc); err != nil {
		t.Fatalf("Create location: %v", err)
	}
	s.LocationID = loc.ID
	s.Status = scheduling.SessionRealizada
	if err := repo.Update(ctx(), s); err != nil {
		t.Fatalf("Update: %v", err)
	}
	n, err := repo.CountRealizadasNoLocalNoPeriodo(ctx(), loc.ID, start.Add(-time.Hour), start.Add(time.Hour))
	if err != nil || n != 1 {
		t.Errorf("CountRealizadas: quero 1, veio %d (%v)", n, err)
	}
}

// --- round-trip de cifra + NÃO-VAZAMENTO ---

func TestAnamnesisCifraRoundTripENaoVazamento(t *testing.T) {
	a := newTestApp(t)
	cipher := testCipher(t)
	pid := seedPatient(t, a, "anam@exemplo.com")
	repo := NewProntuarioRepository(a, cipher)

	const segredo = "segredo clinico"
	an := &clinical.Anamnesis{
		ID:        ulid.New(),
		PatientID: pid,
		Content:   domain.NewSealedText(segredo),
		CreatedAt: clock.Now(),
		UpdatedAt: clock.Now(),
	}
	if err := repo.UpsertAnamnesis(ctx(), an); err != nil {
		t.Fatalf("UpsertAnamnesis: %v", err)
	}

	// Releitura pelo adapter decifra o claro.
	got, err := repo.GetAnamnesis(ctx(), pid)
	if err != nil {
		t.Fatalf("GetAnamnesis: %v", err)
	}
	if got.Content.Reveal() != segredo {
		t.Errorf("claro não round-trip: quero %q, veio %q", segredo, got.Content.Reveal())
	}

	// NÃO-VAZAMENTO: a coluna crua guarda só o envelope, nunca o claro.
	raw, err := a.FindFirstRecordByFilter("anamnesis", "patient_id = {:p}", map[string]any{"p": pid})
	if err != nil {
		t.Fatalf("leitura crua: %v", err)
	}
	env := raw.GetString("conteudo")
	if strings.Contains(env, segredo) {
		t.Errorf("VAZAMENTO: a coluna contém o claro: %q", env)
	}
	if n := strings.Count(env, "."); n != 4 {
		t.Errorf("envelope deveria ter 5 campos (4 pontos), veio %d: %q", n, env)
	}
	if !strings.HasPrefix(env, "v1.aes-256-gcm.") {
		t.Errorf("prefixo de envelope inesperado: %q", env)
	}
}

func TestNoteCifraRoundTripENaoVazamento(t *testing.T) {
	a := newTestApp(t)
	cipher := testCipher(t)
	pid := seedPatient(t, a, "note@exemplo.com")
	repo := NewProntuarioRepository(a, cipher)

	const segredo = "paciente relatou crise de ansiedade"
	n := &clinical.Note{
		ID:        ulid.New(),
		PatientID: pid,
		Content:   domain.NewSealedText(segredo),
		CreatedAt: clock.Now(),
		UpdatedAt: clock.Now(),
	}
	if err := repo.CreateNote(ctx(), n); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	notes, err := repo.ListNotes(ctx(), pid)
	if err != nil || len(notes) != 1 {
		t.Fatalf("ListNotes: %d (%v)", len(notes), err)
	}
	if notes[0].Content.Reveal() != segredo {
		t.Errorf("claro não round-trip: %q", notes[0].Content.Reveal())
	}
	raw, err := a.FindFirstRecordByFilter("note", "patient_id = {:p}", map[string]any{"p": pid})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw.GetString("conteudo"), segredo) {
		t.Errorf("VAZAMENTO: a coluna note.conteudo contém o claro")
	}
}

// --- UNIQUE / idempotência ---

func TestPatientUniqueEmailConflict(t *testing.T) {
	a := newTestApp(t)
	repo := NewPatientRepository(a)

	novo := func(email string) *patient.Patient {
		return &patient.Patient{
			ID: ulid.New(), Name: "X", Phone: "5511999990001",
			Email: email, ApprovalStatus: patient.PatientPending,
			CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
		}
	}
	if err := repo.Create(ctx(), novo("dup@exemplo.com")); err != nil {
		t.Fatalf("1º Create: %v", err)
	}
	err := repo.Create(ctx(), novo("dup@exemplo.com"))
	if err == nil || !errors.Is(err, app.ErrConflict) {
		t.Errorf("2º Create deveria devolver app.ErrConflict, veio: %v", err)
	}
}

func TestDebtInsertIfAbsentIdempotente(t *testing.T) {
	a := newTestApp(t)
	pid := seedPatient(t, a, "debt@exemplo.com")
	repo := NewDebtRepository(a)

	novo := func() *billing.Debt {
		return &billing.Debt{
			ID: ulid.New(), PatientID: pid, Amount: 15000,
			Status: billing.DebtAberto, IdempotencyKey: "session:abc",
			CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
		}
	}
	created, err := repo.InsertIfAbsent(ctx(), novo())
	if err != nil || !created {
		t.Fatalf("1ª inserção: created=%v err=%v", created, err)
	}
	created2, err := repo.InsertIfAbsent(ctx(), novo())
	if err != nil {
		t.Fatalf("2ª inserção erro inesperado: %v", err)
	}
	if created2 {
		t.Error("2ª inserção com mesma idempotency_key deveria devolver created=false")
	}

	got, err := repo.GetByIdempotencyKey(ctx(), "session:abc")
	if err != nil || got.Amount != 15000 {
		t.Errorf("GetByIdempotencyKey: %+v (%v)", got, err)
	}
}

func TestUpsertSessionCostNaoDuplica(t *testing.T) {
	a := newTestApp(t)
	pid := seedPatient(t, a, "scost@exemplo.com")
	sessRepo := NewSessionRepository(a)
	sid := ulid.New()
	if err := sessRepo.Create(ctx(), &scheduling.Session{
		ID: sid, PatientID: pid, Modality: scheduling.ModalityOnline,
		StartsAt: clock.Now(), EndsAt: clock.Now().Add(time.Hour),
		Status: scheduling.SessionAgendada, CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	repo := NewCostRepository(a)
	sc := &costs.SessionCost{
		ID: ulid.New(), SessionID: sid, Amount: 3000,
		Method: costs.CostMethodDireto, CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
	}
	if err := repo.UpsertSessionCost(ctx(), sc); err != nil {
		t.Fatalf("1º upsert: %v", err)
	}
	// Segundo upsert com novo valor: atualiza, não duplica (UNIQUE session_id).
	sc2 := &costs.SessionCost{
		ID: ulid.New(), SessionID: sid, Amount: 4500,
		Method: costs.CostMethodRateio, CreatedAt: clock.Now(), UpdatedAt: clock.Now(),
	}
	if err := repo.UpsertSessionCost(ctx(), sc2); err != nil {
		t.Fatalf("2º upsert: %v", err)
	}
	got, err := repo.GetSessionCost(ctx(), sid)
	if err != nil {
		t.Fatalf("GetSessionCost: %v", err)
	}
	if got.Amount != 4500 || got.Method != costs.CostMethodRateio {
		t.Errorf("upsert não atualizou: %+v", got)
	}
}

// --- soft-delete ---

func TestPatientSoftDelete(t *testing.T) {
	a := newTestApp(t)
	repo := NewPatientRepository(a)
	pid := seedPatient(t, a, "sd@exemplo.com")

	if err := repo.SoftDelete(ctx(), pid); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// Get do removido -> ErrNotFound.
	if _, err := repo.Get(ctx(), pid); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("Get após soft-delete deveria ser ErrNotFound, veio: %v", err)
	}
	// List não retorna o removido.
	list, err := repo.List(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List após soft-delete deveria estar vazia, veio %d", len(list))
	}
}

func TestGetInexistenteNotFound(t *testing.T) {
	a := newTestApp(t)
	repo := NewPatientRepository(a)
	if _, err := repo.Get(ctx(), ulid.New()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("Get de id inexistente deveria ser ErrNotFound, veio: %v", err)
	}
}
