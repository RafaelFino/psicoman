package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func newCost(repo CostRepository, locs LocationRepository, src CostSessionSource, c clock.Clock) *CostService {
	s := NewCostService(repo, locs, src)
	s.clock = c
	return s
}

func costedSession(id, locationID string) *scheduling.Session {
	return &scheduling.Session{
		ID: id, PatientID: "p1", LocationID: locationID, Status: scheduling.SessionRealizada,
		ConsiderCost: true, StartsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()),
	}
}

func TestCostOnSessionFinished_Direto(t *testing.T) {
	ctx := context.Background()
	repo := newCostRepo()
	locs := newLocationRepo()
	locs.byID["loc1"] = &scheduling.Location{ID: "loc1", CostAmount: 8000, CostPeriod: scheduling.PeriodPorSessao}
	svc := newCost(repo, locs, &fakeCostSessionSource{}, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, costedSession("s1", "loc1")); err != nil {
		t.Fatal(err)
	}
	sc := repo.sessionCosts["s1"]
	if sc == nil {
		t.Fatal("custo de sessão não atribuído")
	}
	if sc.Method != costs.CostMethodDireto || sc.Amount != 8000 {
		t.Fatalf("custo direto inesperado: method=%s amount=%d", sc.Method, sc.Amount)
	}
}

func TestCostOnSessionFinished_Rateio(t *testing.T) {
	ctx := context.Background()
	repo := newCostRepo()
	locs := newLocationRepo()
	locs.byID["loc1"] = &scheduling.Location{ID: "loc1", CostAmount: 90000, CostPeriod: scheduling.PeriodMensal}
	src := &fakeCostSessionSource{count: 4}
	svc := newCost(repo, locs, src, fixedAt(2026, 3, 10))

	if err := svc.OnSessionFinished(ctx, costedSession("s1", "loc1")); err != nil {
		t.Fatal(err)
	}
	sc := repo.sessionCosts["s1"]
	if sc == nil {
		t.Fatal("custo de sessão não atribuído")
	}
	if sc.Method != costs.CostMethodRateio || sc.Amount != 90000/4 {
		t.Fatalf("rateio inesperado: method=%s amount=%d", sc.Method, sc.Amount)
	}
	if !strings.Contains(sc.BaseSnapshot, "\"sessions\":4") {
		t.Fatalf("snapshot não registra a base do rateio: %s", sc.BaseSnapshot)
	}
}

func TestCostOnSessionFinished_SemConsiderCost(t *testing.T) {
	ctx := context.Background()
	repo := newCostRepo()
	locs := newLocationRepo()
	locs.byID["loc1"] = &scheduling.Location{ID: "loc1", CostAmount: 8000, CostPeriod: scheduling.PeriodPorSessao}
	svc := newCost(repo, locs, &fakeCostSessionSource{}, fixedAt(2026, 3, 10))

	sess := costedSession("s1", "loc1")
	sess.ConsiderCost = false
	if err := svc.OnSessionFinished(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if repo.upserts != 0 {
		t.Fatalf("consider_cost=false não deveria atribuir custo, upserts=%d", repo.upserts)
	}
}
