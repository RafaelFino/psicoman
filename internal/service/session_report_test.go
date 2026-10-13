package service

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// fakeLocationRepo mínimo para o relatório por local.
type fakeLocationRepo struct{ byID map[string]*domain.Location }

func (f *fakeLocationRepo) Create(context.Context, *domain.Location) error { return nil }
func (f *fakeLocationRepo) Update(context.Context, *domain.Location) error { return nil }
func (f *fakeLocationRepo) Get(_ context.Context, id string) (*domain.Location, error) {
	if l, ok := f.byID[id]; ok {
		return l, nil
	}
	return nil, ErrNotFound
}
func (f *fakeLocationRepo) List(context.Context) ([]*domain.Location, error) { return nil, nil }
func (f *fakeLocationRepo) SoftDelete(context.Context, string) error         { return nil }
func (f *fakeLocationRepo) AddAvailability(context.Context, *domain.Availability) error {
	return nil
}
func (f *fakeLocationRepo) ListAvailability(context.Context, string) ([]*domain.Availability, error) {
	return nil, nil
}
func (f *fakeLocationRepo) DeleteAvailability(context.Context, string) error { return nil }

// metricsDebtRepo: débitos em memória para o faturado.
type metricsDebtRepo struct{ debts []*domain.Debt }

func (f *metricsDebtRepo) InsertIfAbsent(context.Context, *domain.Debt) (bool, error) {
	return true, nil
}
func (f *metricsDebtRepo) Get(context.Context, string) (*domain.Debt, error) { return nil, ErrNotFound }
func (f *metricsDebtRepo) GetByIdempotencyKey(context.Context, string) (*domain.Debt, error) {
	return nil, ErrNotFound
}
func (f *metricsDebtRepo) List(context.Context) ([]*domain.Debt, error) { return f.debts, nil }
func (f *metricsDebtRepo) ListByPatient(context.Context, string) ([]*domain.Debt, error) {
	return f.debts, nil
}
func (f *metricsDebtRepo) ListBySessionIDs(_ context.Context, ids []string) ([]*domain.Debt, error) {
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
func (f *metricsDebtRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *metricsDebtRepo) ListDueBetween(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *metricsDebtRepo) UpdateStatus(context.Context, string, string) error { return nil }
func (f *metricsDebtRepo) SetPDF(context.Context, string, string) error       { return nil }

// variedSessions monta um conjunto com todas as categorias (patient pat-1).
func variedSessions(loc *time.Location) []*domain.Session {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, loc)
	mk := func(id, status, by string, days int) *domain.Session {
		s := &domain.Session{
			ID: id, PatientID: "pat-1", LocationID: "loc-1", Modality: domain.ModalityOnline,
			StartsAt: base.AddDate(0, 0, days), EndsAt: base.AddDate(0, 0, days).Add(time.Hour),
			Status: status,
		}
		if status == domain.SessionCancelada {
			s.CanceledBy = by
		}
		return s
	}
	return []*domain.Session{
		mk("s-real1", domain.SessionRealizada, "", 1),
		mk("s-real2", domain.SessionRealizada, "", 2),
		mk("s-falta", domain.SessionFalta, "", 3),
		mk("s-cpac", domain.SessionCancelada, domain.CancelByPatient, 4),
		mk("s-cter", domain.SessionCancelada, domain.CancelByTherapist, 5),
		mk("s-csem", domain.SessionCancelada, "", 6), // legado: sem autor
		mk("s-ag", domain.SessionAgendada, "", 7),
		mk("s-conf", domain.SessionConfirmada, "", 8),
		mk("s-sol", domain.SessionSolicitada, "", 9),
	}
}

func TestAggregateSessionsCategoriesAndFaturado(t *testing.T) {
	loc := clock.Location()
	sessions := variedSessions(loc)
	// Faturado só das realizadas (uma com débito, outra sem).
	faturado := map[string]int64{"s-real1": 15000, "s-falta": 9999 /* não deve contar */}
	agg := aggregateSessions(sessions, faturado)

	wantCounts := map[string]int{
		domain.CategoryRealizada:          2,
		domain.CategoryFalta:              1,
		domain.CategoryCanceladaPaciente:  1,
		domain.CategoryCanceladaTerapeuta: 1,
		domain.CategoryCanceladaSemAutor:  1,
		domain.CategoryAgendadaConfirmada: 2, // agendada + confirmada
		domain.CategorySolicitada:         1,
	}
	for cat, want := range wantCounts {
		if agg.Counts[cat] != want {
			t.Errorf("categoria %q = %d, quer %d", cat, agg.Counts[cat], want)
		}
	}
	if agg.Total != 9 {
		t.Errorf("total = %d, quer 9", agg.Total)
	}
	// Faturado = só a realizada com débito (15000); a falta não entra.
	if agg.FaturadoReal != 15000 {
		t.Errorf("faturado = %d, quer 15000 (só realizadas)", agg.FaturadoReal)
	}
}

func TestRatesZeroSafe(t *testing.T) {
	// Sem sessões: taxas = 0, sem divisão por zero.
	empty := map[string]int{}
	if attendanceRate(empty) != 0 {
		t.Error("comparecimento vazio deveria ser 0")
	}
	if cancelRate(empty, 0) != 0 {
		t.Error("cancelamento vazio deveria ser 0")
	}

	// comparecimento = 2 / (2 + 1 + 1) = 0.5; cancelamento = 3/9.
	counts := map[string]int{
		domain.CategoryRealizada:          2,
		domain.CategoryFalta:              1,
		domain.CategoryCanceladaPaciente:  1,
		domain.CategoryCanceladaTerapeuta: 1,
		domain.CategoryCanceladaSemAutor:  1,
	}
	if got := attendanceRate(counts); got != 0.5 {
		t.Errorf("comparecimento = %v, quer 0.5", got)
	}
	if got := cancelRate(counts, 9); got < 0.333 || got > 0.334 {
		t.Errorf("cancelamento = %v, quer ~0.333", got)
	}
}

func TestPatientSessionMetricsService(t *testing.T) {
	loc := clock.Location()
	repo := newFakeSessionRepo()
	for _, s := range variedSessions(loc) {
		repo.put(s)
	}
	debts := &metricsDebtRepo{debts: []*domain.Debt{
		{ID: "d1", PatientID: "pat-1", SessionID: "s-real1", Amount: 15000},
		{ID: "d2", PatientID: "pat-1", SessionID: "s-real2", Amount: 20000},
	}}
	svc := NewSessionMetricsService(repo, debts, &fakeLocationRepo{})

	m, err := svc.PatientSessionMetrics(context.Background(), "pat-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("PatientSessionMetrics: %v", err)
	}
	if m.Total != 9 {
		t.Errorf("total = %d, quer 9", m.Total)
	}
	if m.FaturadoReal != 35000 {
		t.Errorf("faturado = %d, quer 35000", m.FaturadoReal)
	}
	if m.TaxaComparecimento != 0.5 {
		t.Errorf("comparecimento = %v, quer 0.5", m.TaxaComparecimento)
	}
}

func TestLocationSessionReportService(t *testing.T) {
	loc := clock.Location()
	repo := newFakeSessionRepo()
	for _, s := range variedSessions(loc) {
		repo.put(s)
	}
	debts := &metricsDebtRepo{debts: []*domain.Debt{
		{ID: "d1", PatientID: "pat-1", SessionID: "s-real1", Amount: 15000},
	}}
	locations := &fakeLocationRepo{byID: map[string]*domain.Location{
		"loc-1": {ID: "loc-1", Name: "Consultório"},
	}}
	svc := NewSessionMetricsService(repo, debts, locations)

	// Histórico completo (from/to zero).
	rep, err := svc.LocationSessionReport(context.Background(), "loc-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("LocationSessionReport: %v", err)
	}
	if rep.LocationName != "Consultório" {
		t.Errorf("nome do local = %q", rep.LocationName)
	}
	if rep.Total != 9 {
		t.Errorf("total = %d, quer 9", rep.Total)
	}
	if rep.Counts[domain.CategoryRealizada] != 2 {
		t.Errorf("realizadas = %d, quer 2", rep.Counts[domain.CategoryRealizada])
	}
	if rep.FaturadoReal != 15000 {
		t.Errorf("faturado = %d, quer 15000", rep.FaturadoReal)
	}

	// Recorte que exclui tudo (janela no passado distante).
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, loc)
	to := time.Date(2020, 2, 1, 0, 0, 0, 0, loc)
	rep2, _ := svc.LocationSessionReport(context.Background(), "loc-1", from, to)
	if rep2.Total != 0 {
		t.Errorf("recorte vazio total = %d, quer 0", rep2.Total)
	}
}
