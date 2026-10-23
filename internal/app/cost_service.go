package app

import (
	"context"
	"errors"

	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// CostService atribui custo a sessões (direto ou rateio) e faz o CRUD de
// categorias/itens. As fórmulas de rateio/período vivem no domínio costs; o
// serviço coordena o I/O (local, contagem de sessões, persistência).
// Portado de c3ab585:internal/service/cost.go.
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
func (s *CostService) OnSessionFinished(ctx context.Context, sess *scheduling.Session) error {
	if !sess.ConsiderCost || sess.LocationID == "" {
		return nil
	}
	loc, err := s.locations.Get(ctx, sess.LocationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	now := s.clock.Now()
	var sc *costs.SessionCost
	if loc.CostPeriod == scheduling.PeriodPorSessao {
		sc = costs.DirectCost(sess.ID, loc.CostAmount, loc.CostPeriod, now)
	} else {
		de, ate := costs.PeriodBounds(loc.CostPeriod, sess.StartsAt)
		count, err := s.sessions.CountRealizadasNoLocalNoPeriodo(ctx, loc.ID, de, ate)
		if err != nil {
			return err
		}
		// count<1 é tratado como 1 pelo construtor (a própria sessão finalizada).
		sc = costs.RateioCost(sess.ID, loc.CostAmount, loc.CostPeriod, count, de, ate, now)
	}
	return s.repo.UpsertSessionCost(ctx, sc)
}

// --- CRUD de custos ---

// CreateCategory cria uma categoria de custo.
func (s *CostService) CreateCategory(ctx context.Context, kind, name string) (*costs.CostCategory, error) {
	now := s.clock.Now()
	c := &costs.CostCategory{ID: ulid.New(), Kind: kind, Name: name, CreatedAt: now, UpdatedAt: now}
	if err := c.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.repo.CreateCategory(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ListCategories devolve as categorias.
func (s *CostService) ListCategories(ctx context.Context) ([]*costs.CostCategory, error) {
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
func (s *CostService) CreateItem(ctx context.Context, in CostItemInput) (*costs.CostItem, error) {
	now := s.clock.Now()
	i := &costs.CostItem{
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
func (s *CostService) ListItems(ctx context.Context) ([]*costs.CostItem, error) {
	return s.repo.ListItems(ctx)
}

// DeleteItem remove um item de custo.
func (s *CostService) DeleteItem(ctx context.Context, id string) error {
	return s.repo.SoftDeleteItem(ctx, id)
}

// GetSessionCost devolve o custo atribuído a uma sessão.
func (s *CostService) GetSessionCost(ctx context.Context, sessionID string) (*costs.SessionCost, error) {
	return s.repo.GetSessionCost(ctx, sessionID)
}
