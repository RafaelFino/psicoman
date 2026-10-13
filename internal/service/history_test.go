package service

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// --- fakes dedicados ao teste de histórico -----------------------------------

type histProntuarioRepo struct {
	notes []*domain.Note
	anam  *domain.Anamnesis
}

func (f *histProntuarioRepo) GetAnamnesis(_ context.Context, _ string) (*domain.Anamnesis, error) {
	if f.anam == nil {
		return nil, ErrNotFound
	}
	return f.anam, nil
}
func (f *histProntuarioRepo) UpsertAnamnesis(context.Context, *domain.Anamnesis) error { return nil }
func (f *histProntuarioRepo) CreateNote(context.Context, *domain.Note) error           { return nil }
func (f *histProntuarioRepo) ListNotes(_ context.Context, _ string) ([]*domain.Note, error) {
	return f.notes, nil
}
func (f *histProntuarioRepo) DeleteNote(context.Context, string) error { return nil }

type histDebtRepo struct{ debts []*domain.Debt }

func (f *histDebtRepo) InsertIfAbsent(context.Context, *domain.Debt) (bool, error) { return true, nil }
func (f *histDebtRepo) Get(context.Context, string) (*domain.Debt, error)          { return nil, ErrNotFound }
func (f *histDebtRepo) GetByIdempotencyKey(context.Context, string) (*domain.Debt, error) {
	return nil, ErrNotFound
}
func (f *histDebtRepo) List(context.Context) ([]*domain.Debt, error) { return f.debts, nil }
func (f *histDebtRepo) ListByPatient(_ context.Context, _ string) ([]*domain.Debt, error) {
	return f.debts, nil
}
func (f *histDebtRepo) ListBySessionIDs(_ context.Context, ids []string) ([]*domain.Debt, error) {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	var out []*domain.Debt
	for _, d := range f.debts {
		if set[d.SessionID] {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *histDebtRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *histDebtRepo) ListDueBetween(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *histDebtRepo) UpdateStatus(context.Context, string, string) error { return nil }
func (f *histDebtRepo) SetPDF(context.Context, string, string) error       { return nil }

type histPaymentRepo struct{ byDebt map[string][]*domain.Payment }

func (f *histPaymentRepo) Insert(context.Context, *domain.Payment) error { return nil }
func (f *histPaymentRepo) ListByDebt(_ context.Context, debtID string) ([]*domain.Payment, error) {
	return f.byDebt[debtID], nil
}
func (f *histPaymentRepo) SumByDebt(_ context.Context, debtID string) (int64, error) {
	var sum int64
	for _, p := range f.byDebt[debtID] {
		sum += p.Amount
	}
	return sum, nil
}
func (f *histPaymentRepo) ListByRange(context.Context, time.Time, time.Time) ([]*domain.Payment, error) {
	return nil, nil
}

type histGEDRepo struct{ files []*domain.GEDFile }

func (f *histGEDRepo) Insert(context.Context, *domain.GEDFile) error        { return nil }
func (f *histGEDRepo) Get(context.Context, string) (*domain.GEDFile, error) { return nil, ErrNotFound }
func (f *histGEDRepo) FindByHash(context.Context, string, string) (*domain.GEDFile, error) {
	return nil, ErrNotFound
}
func (f *histGEDRepo) ListByPatient(_ context.Context, _ string) ([]*domain.GEDFile, error) {
	return f.files, nil
}
func (f *histGEDRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.GEDFile, error) {
	return nil, nil
}
func (f *histGEDRepo) Delete(context.Context, string) error { return nil }

// --- teste --------------------------------------------------------------------

func TestPatientHistoryComposition(t *testing.T) {
	loc := clock.Location()
	base := time.Date(2026, 3, 1, 9, 0, 0, 0, loc)

	patients := newFakePatientRepo()
	patients.put(&domain.Patient{
		ID: "pat-1", Name: "Ana", Email: "ana@x.com",
		ApprovalStatus: domain.PatientApproved,
		CreatedAt:      base, UpdatedAt: base.AddDate(0, 0, 1),
	})

	sessions := newFakeSessionRepo()
	sessions.put(&domain.Session{
		ID: "sess-1", PatientID: "pat-1", Modality: domain.ModalityOnline,
		StartsAt: base.AddDate(0, 0, 5), EndsAt: base.AddDate(0, 0, 5).Add(time.Hour),
		Status: domain.SessionRealizada, CreatedAt: base.AddDate(0, 0, 2),
	})
	sessions.put(&domain.Session{
		ID: "sess-2", PatientID: "pat-1", Modality: domain.ModalityPresencial,
		StartsAt: base.AddDate(0, 0, 10), EndsAt: base.AddDate(0, 0, 10).Add(time.Hour),
		Status: domain.SessionCancelada, CanceledBy: domain.CancelByPatient, CancelReason: "imprevisto",
		CanceledAt: base.AddDate(0, 0, 9), CreatedAt: base.AddDate(0, 0, 3),
	})

	notes := &histProntuarioRepo{
		notes: []*domain.Note{
			{ID: "n-1", PatientID: "pat-1", SessionID: "sess-1", Content: "registro", CreatedAt: base.AddDate(0, 0, 5)},
			{ID: "n-2", PatientID: "pat-1", Content: "nota livre", CreatedAt: base.AddDate(0, 0, 6)},
		},
		anam: &domain.Anamnesis{ID: "a-1", PatientID: "pat-1", Content: "história", CreatedAt: base, UpdatedAt: base.AddDate(0, 0, 1)},
	}

	debts := &histDebtRepo{debts: []*domain.Debt{
		{ID: "d-1", PatientID: "pat-1", SessionID: "sess-1", Amount: 15000, Status: domain.DebtAberto, CreatedAt: base.AddDate(0, 0, 5)},
	}}
	payments := &histPaymentRepo{byDebt: map[string][]*domain.Payment{
		"d-1": {{ID: "p-1", DebtID: "d-1", Amount: 15000, Method: "pix", PaidAt: base.AddDate(0, 0, 7)}},
	}}
	ged := &histGEDRepo{files: []*domain.GEDFile{
		{ID: "f-1", PatientID: "pat-1", MIME: "application/pdf", CreatedAt: base.AddDate(0, 0, 8)},
	}}

	audit := &capturingAuditRepo{}
	// Remarcação registrada no audit da sessão 1.
	audit.entries = append(audit.entries, &domain.AuditLog{
		ID: "au-1", Action: domain.AuditActionSessionReschedule, Entity: "session", EntityID: "sess-1",
		Metadata:  map[string]any{"session_id": "sess-1", "from": "2026-03-04T09:00:00-03:00", "to": "2026-03-06T09:00:00-03:00"},
		CreatedAt: base.AddDate(0, 0, 4),
	})

	svc := NewHistoryService(patients, sessions, notes, debts, payments, ged, NewAuditService(audit))
	items, err := svc.PatientHistory(context.Background(), "pat-1")
	if err != nil {
		t.Fatalf("PatientHistory: %v", err)
	}

	// Presença de cada tipo esperado.
	counts := map[string]int{}
	for _, it := range items {
		counts[it.Tipo]++
	}
	for _, tipo := range []string{
		HistoryCadastro, HistoryAprovacao, HistorySessao, HistoryRemarcacao,
		HistoryRegistro, HistoryNotaLivre, HistoryAnamnese, HistoryDebito,
		HistoryPagamento, HistoryArquivo,
	} {
		if counts[tipo] == 0 {
			t.Errorf("tipo %q ausente no histórico", tipo)
		}
	}
	if counts[HistorySessao] != 2 {
		t.Errorf("sessões = %d, quer 2", counts[HistorySessao])
	}

	// Ordenação desc por data.
	for i := 1; i < len(items); i++ {
		if items[i-1].Data.Before(items[i].Data) {
			t.Errorf("ordem não-desc entre %d (%v) e %d (%v)", i-1, items[i-1].Data, i, items[i].Data)
		}
	}

	// Cancelamento mostra autor + motivo no resumo.
	var canc *HistoryItem
	for i := range items {
		if items[i].Ref.ID == "sess-2" && items[i].Tipo == HistorySessao {
			canc = &items[i]
		}
	}
	if canc == nil {
		t.Fatal("sessão cancelada ausente")
	}
	if canc.Resumo == "" || !contains(canc.Resumo, "paciente") || !contains(canc.Resumo, "imprevisto") {
		t.Errorf("resumo do cancelamento não traz autor/motivo: %q", canc.Resumo)
	}

	// Auditoria de leitura registrada, sem conteúdo clínico em metadata.
	readFound := false
	for _, e := range audit.entries {
		if e.Action == domain.AuditActionRead && e.Entity == "patient_history" {
			readFound = true
			for k, v := range e.Metadata {
				if s, ok := v.(string); ok && (contains(s, "história") || contains(s, "registro") || contains(s, "nota livre")) {
					t.Errorf("metadata de auditoria vazou conteúdo clínico em %q=%q", k, s)
				}
			}
		}
	}
	if !readFound {
		t.Error("esperava audit AuditActionRead para patient_history")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
