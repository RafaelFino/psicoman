package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// CostRepository persiste categorias, itens e custos de sessão.
type CostRepository interface {
	CreateCategory(ctx context.Context, c *domain.CostCategory) error
	ListCategories(ctx context.Context) ([]*domain.CostCategory, error)
	CreateItem(ctx context.Context, i *domain.CostItem) error
	ListItems(ctx context.Context) ([]*domain.CostItem, error)
	ListItemsByOrigin(ctx context.Context, originID string) ([]*domain.CostItem, error)
	SoftDeleteItem(ctx context.Context, id string) error

	UpsertSessionCost(ctx context.Context, sc *domain.SessionCost) error
	GetSessionCost(ctx context.Context, sessionID string) (*domain.SessionCost, error)
}

// CostSessionSource fornece as sessões realizadas de um local num período,
// para o rateio proporcional (implementado pelo SessionRepository via adapter).
type CostSessionSource interface {
	// CountRealizadasNoLocalNoPeriodo conta sessões realizadas no local entre
	// [de, ate), considerando apenas as que computam custo.
	CountRealizadasNoLocalNoPeriodo(ctx context.Context, locationID string, de, ate time.Time) (int, error)
}

// CostService atribui custo a sessões (direto ou rateio) e produz relatórios.
type CostService struct {
	repo      CostRepository
	locations LocationRepository
	sessions  CostSessionSource
	clock     clock.Clock
}

// NewCostService cria o serviço de custos.
func NewCostService(repo CostRepository, locations LocationRepository, sessions CostSessionSource) *CostService {
	return &CostService{repo: repo, locations: locations, sessions: sessions, clock: clock.System{}}
}

var _ SessionFinishedHook = (*CostService)(nil)

// OnSessionFinished atribui custo à sessão quando consider_cost está marcado.
// custo por_sessao → direto; diario/mensal/anual → rateio proporcional entre as
// sessões realizadas no local no período (docs/architecture.md §4.1).
func (s *CostService) OnSessionFinished(ctx context.Context, sess *domain.Session) error {
	if !sess.ConsiderCost || sess.LocationID == "" {
		return nil
	}
	loc, err := s.locations.Get(ctx, sess.LocationID)
	if err != nil {
		if err == ErrNotFound {
			return nil
		}
		return err
	}

	now := s.clock.Now()
	sc := &domain.SessionCost{
		ID: ulid.New(), SessionID: sess.ID, CreatedAt: now, UpdatedAt: now,
	}

	if loc.CostPeriod == domain.PeriodPorSessao {
		sc.Method = domain.CostMethodDireto
		sc.Amount = loc.CostAmount
		sc.BaseSnapshot = snapshot(map[string]any{"cost_period": loc.CostPeriod, "cost_amount": loc.CostAmount})
	} else {
		de, ate := periodBounds(loc.CostPeriod, sess.StartsAt)
		count, err := s.sessions.CountRealizadasNoLocalNoPeriodo(ctx, loc.ID, de, ate)
		if err != nil {
			return err
		}
		if count < 1 {
			count = 1 // a própria sessão sendo finalizada
		}
		sc.Method = domain.CostMethodRateio
		sc.Amount = loc.CostAmount / int64(count)
		sc.BaseSnapshot = snapshot(map[string]any{
			"cost_period":  loc.CostPeriod,
			"cost_amount":  loc.CostAmount,
			"sessions":     count,
			"period_start": clock.Format(de),
			"period_end":   clock.Format(ate),
		})
	}
	return s.repo.UpsertSessionCost(ctx, sc)
}

// periodBounds devolve [início, fim) do período que contém ref, conforme a
// periodicidade do custo.
func periodBounds(period string, ref time.Time) (time.Time, time.Time) {
	ref = ref.In(clock.Location())
	switch period {
	case domain.PeriodDiario:
		start := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(0, 0, 1)
	case domain.PeriodMensal:
		start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(0, 1, 0)
	case domain.PeriodAnual:
		start := time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, clock.Location())
		return start, start.AddDate(1, 0, 0)
	default:
		return ref, ref
	}
}

func snapshot(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// --- CRUD de custos ---

// CreateCategory cria uma categoria de custo.
func (s *CostService) CreateCategory(ctx context.Context, kind, name string) (*domain.CostCategory, error) {
	now := s.clock.Now()
	c := &domain.CostCategory{ID: ulid.New(), Kind: kind, Name: name, CreatedAt: now, UpdatedAt: now}
	if err := c.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ListCategories devolve as categorias.
func (s *CostService) ListCategories(ctx context.Context) ([]*domain.CostCategory, error) {
	return s.repo.ListCategories(ctx)
}

// CostItemInput são os dados de um item de custo.
type CostItemInput struct {
	CategoryID string
	Name       string
	Amount     int64
	Period     string
	OriginID   string
	LocationID string
}

// CreateItem cria um item de custo.
func (s *CostService) CreateItem(ctx context.Context, in CostItemInput) (*domain.CostItem, error) {
	now := s.clock.Now()
	i := &domain.CostItem{
		ID: ulid.New(), CategoryID: in.CategoryID, Name: in.Name, Amount: in.Amount,
		Period: in.Period, OriginID: in.OriginID, LocationID: in.LocationID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := i.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateItem(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

// ListItems devolve os itens de custo.
func (s *CostService) ListItems(ctx context.Context) ([]*domain.CostItem, error) {
	return s.repo.ListItems(ctx)
}

// DeleteItem remove um item de custo.
func (s *CostService) DeleteItem(ctx context.Context, id string) error {
	return s.repo.SoftDeleteItem(ctx, id)
}

// GetSessionCost devolve o custo atribuído a uma sessão.
func (s *CostService) GetSessionCost(ctx context.Context, sessionID string) (*domain.SessionCost, error) {
	return s.repo.GetSessionCost(ctx, sessionID)
}
