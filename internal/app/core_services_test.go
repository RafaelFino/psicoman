package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// --- LocationService ---

// trackingLocationRepo guarda locais e disponibilidades em memória.
type trackingLocationRepo struct {
	byID   map[string]*scheduling.Location
	avail  map[string][]*scheduling.Availability
	availN int
}

func newTrackingLocationRepo() *trackingLocationRepo {
	return &trackingLocationRepo{byID: map[string]*scheduling.Location{}, avail: map[string][]*scheduling.Availability{}}
}

func (f *trackingLocationRepo) Create(_ context.Context, l *scheduling.Location) error {
	f.byID[l.ID] = l
	return nil
}
func (f *trackingLocationRepo) Update(_ context.Context, l *scheduling.Location) error {
	f.byID[l.ID] = l
	return nil
}
func (f *trackingLocationRepo) Get(_ context.Context, id string) (*scheduling.Location, error) {
	l, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}
func (f *trackingLocationRepo) List(_ context.Context) ([]*scheduling.Location, error) {
	out := make([]*scheduling.Location, 0, len(f.byID))
	for _, l := range f.byID {
		out = append(out, l)
	}
	return out, nil
}
func (f *trackingLocationRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}
func (f *trackingLocationRepo) AddAvailability(_ context.Context, a *scheduling.Availability) error {
	f.availN++
	f.avail[a.LocationID] = append(f.avail[a.LocationID], a)
	return nil
}
func (f *trackingLocationRepo) ListAvailability(_ context.Context, locationID string) ([]*scheduling.Availability, error) {
	return f.avail[locationID], nil
}
func (f *trackingLocationRepo) DeleteAvailability(context.Context, string) error { return nil }

func TestLocationServiceCreateEValida(t *testing.T) {
	ctx := context.Background()
	repo := newTrackingLocationRepo()
	svc := NewLocationService(repo)

	l, err := svc.Create(ctx, LocationInput{Name: "Consultório", Modality: "presencial", CostAmount: 50000, CostPeriod: "mensal"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if l.ID == "" || repo.byID[l.ID] == nil {
		t.Fatalf("local não persistido: %+v", l)
	}

	// Modalidade inválida → ErrValidation.
	if _, err := svc.Create(ctx, LocationInput{Name: "X", Modality: "hibrido", CostPeriod: "mensal"}); !errors.Is(err, ErrValidation) {
		t.Errorf("modalidade inválida deveria ser ErrValidation, veio: %v", err)
	}
}

func TestLocationServiceAvailability(t *testing.T) {
	ctx := context.Background()
	repo := newTrackingLocationRepo()
	svc := NewLocationService(repo)
	l, _ := svc.Create(ctx, LocationInput{Name: "C", Modality: "presencial", CostPeriod: "mensal"})

	// Disponibilidade válida.
	a, err := svc.AddAvailability(ctx, l.ID, AvailabilityInput{Weekday: 1, StartTime: "08:00", EndTime: "12:00", Capacity: 1})
	if err != nil {
		t.Fatalf("AddAvailability: %v", err)
	}
	if a.ID == "" {
		t.Fatal("disponibilidade sem id")
	}
	// Horário inválido → ErrValidation.
	if _, err := svc.AddAvailability(ctx, l.ID, AvailabilityInput{Weekday: 1, StartTime: "13:00", EndTime: "09:00", Capacity: 1}); !errors.Is(err, ErrValidation) {
		t.Errorf("horário invertido deveria ser ErrValidation, veio: %v", err)
	}
	// Local inexistente → ErrNotFound.
	if _, err := svc.AddAvailability(ctx, "nao-existe", AvailabilityInput{Weekday: 1, StartTime: "08:00", EndTime: "12:00", Capacity: 1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("local inexistente deveria ser ErrNotFound, veio: %v", err)
	}
}

// --- OriginService ---

type trackingOriginRepo struct {
	byID map[string]*patient.Origin
}

func newTrackingOriginRepo() *trackingOriginRepo {
	return &trackingOriginRepo{byID: map[string]*patient.Origin{}}
}

func (f *trackingOriginRepo) Create(_ context.Context, o *patient.Origin) error {
	f.byID[o.ID] = o
	return nil
}
func (f *trackingOriginRepo) Update(_ context.Context, o *patient.Origin) error {
	f.byID[o.ID] = o
	return nil
}
func (f *trackingOriginRepo) Get(_ context.Context, id string) (*patient.Origin, error) {
	o, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}
func (f *trackingOriginRepo) List(_ context.Context) ([]*patient.Origin, error) {
	out := make([]*patient.Origin, 0, len(f.byID))
	for _, o := range f.byID {
		out = append(out, o)
	}
	return out, nil
}
func (f *trackingOriginRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

func TestOriginServiceCreateEValida(t *testing.T) {
	ctx := context.Background()
	svc := NewOriginService(newTrackingOriginRepo())
	o, err := svc.Create(ctx, "Doctoralia")
	if err != nil || o.ID == "" {
		t.Fatalf("Create: %v / %+v", err, o)
	}
	if _, err := svc.Create(ctx, "   "); !errors.Is(err, ErrValidation) {
		t.Errorf("nome vazio deveria ser ErrValidation, veio: %v", err)
	}
}

// --- PlanService ---

type trackingPlanRepo struct {
	byID      map[string]*billing.Plan
	byPatient map[string][]*billing.Plan
}

func newTrackingPlanRepo() *trackingPlanRepo {
	return &trackingPlanRepo{byID: map[string]*billing.Plan{}, byPatient: map[string][]*billing.Plan{}}
}

func (f *trackingPlanRepo) Create(_ context.Context, p *billing.Plan) error {
	f.byID[p.ID] = p
	f.byPatient[p.PatientID] = append(f.byPatient[p.PatientID], p)
	return nil
}
func (f *trackingPlanRepo) Update(context.Context, *billing.Plan) error { return nil }
func (f *trackingPlanRepo) Get(_ context.Context, id string) (*billing.Plan, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}
func (f *trackingPlanRepo) GetActiveByPatient(_ context.Context, patientID string, ref time.Time) (*billing.Plan, error) {
	for _, p := range f.byPatient[patientID] {
		if p.ActiveOn(ref) {
			return p, nil
		}
	}
	return nil, ErrNotFound
}
func (f *trackingPlanRepo) ListByPatient(_ context.Context, patientID string) ([]*billing.Plan, error) {
	return f.byPatient[patientID], nil
}
func (f *trackingPlanRepo) ListFixedCycleActive(context.Context, time.Time) ([]*billing.Plan, error) {
	return nil, nil
}
func (f *trackingPlanRepo) SoftDelete(context.Context, string) error { return nil }

func TestPlanServiceCreateEValida(t *testing.T) {
	ctx := context.Background()
	svc := NewPlanService(newTrackingPlanRepo())
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, clock.Location())

	// Plano por consulta válido.
	p, err := svc.Create(ctx, PlanInput{
		PatientID: "p1", Type: billing.PlanPorConsulta, Amount: 15000, StartsAt: start,
	})
	if err != nil || p.ID == "" {
		t.Fatalf("Create: %v / %+v", err, p)
	}
	if !p.GeneratesPerSessionDebt() {
		t.Error("plano por consulta deveria gerar débito por sessão")
	}

	// Plano fechado sem valor → ErrValidation.
	if _, err := svc.Create(ctx, PlanInput{PatientID: "p1", Type: billing.PlanFechadoMensal, Amount: 0, DueDay: 10, StartsAt: start}); !errors.Is(err, ErrValidation) {
		t.Errorf("plano fechado sem valor deveria ser ErrValidation, veio: %v", err)
	}

	// Tipo inválido → ErrValidation.
	if _, err := svc.Create(ctx, PlanInput{PatientID: "p1", Type: "inexistente", StartsAt: start}); !errors.Is(err, ErrValidation) {
		t.Errorf("tipo inválido deveria ser ErrValidation, veio: %v", err)
	}

	// ListByPatient devolve o plano criado.
	ps, _ := svc.ListByPatient(ctx, "p1")
	if len(ps) != 1 {
		t.Errorf("ListByPatient: quero 1, veio %d", len(ps))
	}
}
