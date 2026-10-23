package pb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// costRepo implementa app.CostRepository sobre cost_category, cost_item e
// session_cost.
type costRepo struct{ app core.App }

var _ app.CostRepository = (*costRepo)(nil)

// NewCostRepository cria o repositório de custos.
func NewCostRepository(a core.App) app.CostRepository { return &costRepo{app: a} }

func categoryFromRecord(rec *core.Record) *costs.CostCategory {
	return &costs.CostCategory{
		ID:        rec.Id,
		Kind:      rec.GetString("kind"),
		Name:      rec.GetString("name"),
		CreatedAt: parseTime(rec.GetString("created_at")),
		UpdatedAt: parseTime(rec.GetString("updated_at")),
	}
}

func (r *costRepo) CreateCategory(ctx context.Context, c *costs.CostCategory) error {
	col, err := r.app.FindCollectionByNameOrId("cost_category")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	rec.Set("id", c.ID)
	rec.Set("kind", c.Kind)
	rec.Set("name", c.Name)
	rec.Set("created_at", formatTime(c.CreatedAt))
	rec.Set("updated_at", formatTime(c.UpdatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *costRepo) ListCategories(ctx context.Context) ([]*costs.CostCategory, error) {
	recs, err := r.app.FindRecordsByFilter("cost_category", "", "name", 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*costs.CostCategory, 0, len(recs))
	for _, rec := range recs {
		out = append(out, categoryFromRecord(rec))
	}
	return out, nil
}

func itemFromRecord(rec *core.Record) *costs.CostItem {
	return &costs.CostItem{
		ID:         rec.Id,
		CategoryID: rec.GetString("category_id"),
		Name:       rec.GetString("name"),
		Amount:     rec.GetInt64("amount"),
		Period:     rec.GetString("period"),
		OriginID:   rec.GetString("origin_id"),
		LocationID: rec.GetString("location_id"),
		CreatedAt:  parseTime(rec.GetString("created_at")),
		UpdatedAt:  parseTime(rec.GetString("updated_at")),
	}
}

func (r *costRepo) toItemRecord(rec *core.Record, i *costs.CostItem) {
	rec.Set("id", i.ID)
	rec.Set("category_id", i.CategoryID)
	rec.Set("name", i.Name)
	rec.Set("amount", i.Amount)
	rec.Set("period", i.Period)
	rec.Set("origin_id", i.OriginID)
	rec.Set("location_id", i.LocationID)
	rec.Set("created_at", formatTime(i.CreatedAt))
	rec.Set("updated_at", formatTime(i.UpdatedAt))
}

func (r *costRepo) CreateItem(ctx context.Context, i *costs.CostItem) error {
	col, err := r.app.FindCollectionByNameOrId("cost_item")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toItemRecord(rec, i)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *costRepo) ListItems(ctx context.Context) ([]*costs.CostItem, error) {
	return r.listItemsByFilter(notDeletedFilter, nil)
}

func (r *costRepo) ListItemsByOrigin(ctx context.Context, originID string) ([]*costs.CostItem, error) {
	return r.listItemsByFilter("origin_id = {:o} && "+notDeletedFilter, dbxParams("o", originID))
}

func (r *costRepo) SoftDeleteItem(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "cost_item", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *costRepo) listItemsByFilter(filter string, params dbx.Params) ([]*costs.CostItem, error) {
	var recs []*core.Record
	var err error
	if params == nil {
		recs, err = r.app.FindRecordsByFilter("cost_item", filter, "name", 0, 0)
	} else {
		recs, err = r.app.FindRecordsByFilter("cost_item", filter, "name", 0, 0, params)
	}
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*costs.CostItem, 0, len(recs))
	for _, rec := range recs {
		out = append(out, itemFromRecord(rec))
	}
	return out, nil
}

func sessionCostFromRecord(rec *core.Record) *costs.SessionCost {
	return &costs.SessionCost{
		ID:           rec.Id,
		SessionID:    rec.GetString("session_id"),
		Amount:       rec.GetInt64("amount"),
		Method:       rec.GetString("method"),
		BaseSnapshot: rec.GetString("base_snapshot"),
		CreatedAt:    parseTime(rec.GetString("created_at")),
		UpdatedAt:    parseTime(rec.GetString("updated_at")),
	}
}

// UpsertSessionCost grava o custo da sessão respeitando o UNIQUE session_id
// (1:1 com a sessão): atualiza o registro existente ou cria um novo.
func (r *costRepo) UpsertSessionCost(ctx context.Context, sc *costs.SessionCost) error {
	rec, err := r.app.FindFirstRecordByFilter("session_cost", "session_id = {:s}", dbxParams("s", sc.SessionID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return mapError(err)
	}
	if rec == nil {
		col, cerr := r.app.FindCollectionByNameOrId("session_cost")
		if cerr != nil {
			return mapError(cerr)
		}
		rec = core.NewRecord(col)
		rec.Set("id", sc.ID)
		rec.Set("created_at", formatTime(sc.CreatedAt))
	}
	rec.Set("session_id", sc.SessionID)
	rec.Set("amount", sc.Amount)
	rec.Set("method", sc.Method)
	rec.Set("base_snapshot", sc.BaseSnapshot)
	rec.Set("updated_at", formatTime(sc.UpdatedAt))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *costRepo) GetSessionCost(ctx context.Context, sessionID string) (*costs.SessionCost, error) {
	rec, err := r.app.FindFirstRecordByFilter("session_cost", "session_id = {:s}", dbxParams("s", sessionID))
	if err != nil {
		return nil, mapError(err)
	}
	return sessionCostFromRecord(rec), nil
}

// reportRepo implementa app.ReportRepository com agregações por período. As
// semânticas são portadas de c3ab585:internal/repository/sqlite/report.go
// (paridade com o legado); as somas são feitas em Go sobre os records.
type reportRepo struct{ app core.App }

var _ app.ReportRepository = (*reportRepo)(nil)

// NewReportRepository cria o repositório de relatórios.
func NewReportRepository(a core.App) app.ReportRepository { return &reportRepo{app: a} }

// RevenueByOrigin soma os pagamentos recebidos no período, agrupados pela origem
// do paciente do débito quitado (payment -> debt -> patient.origin_id).
func (r *reportRepo) RevenueByOrigin(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	payments, err := r.app.FindRecordsByFilter("payment",
		"paid_at >= {:from} && paid_at < {:to}", "", 0, 0,
		dbx.Params{"from": clock.Format(de), "to": clock.Format(ate)})
	if err != nil {
		return nil, mapError(err)
	}
	// Cache debt->origin para evitar buscas repetidas.
	originByDebt := map[string]string{}
	out := map[string]int64{}
	for _, pay := range payments {
		debtID := pay.GetString("debt_id")
		originID, ok := originByDebt[debtID]
		if !ok {
			originID, err = r.originForDebt(debtID)
			if err != nil {
				return nil, err
			}
			originByDebt[debtID] = originID
		}
		out[originID] += pay.GetInt64("amount")
	}
	return out, nil
}

// originForDebt resolve a origem do paciente dono do débito ("" se ausente).
func (r *reportRepo) originForDebt(debtID string) (string, error) {
	debt, err := r.app.FindRecordById("debt", debtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	pt, err := r.app.FindRecordById("patient", debt.GetString("patient_id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return pt.GetString("origin_id"), nil
}

// DebtsSummary agrega débitos gerados, em aberto, recebidos e vencidos no período.
func (r *reportRepo) DebtsSummary(ctx context.Context, de, ate time.Time) (*costs.FinancialSummary, error) {
	deStr, ateStr := clock.Format(de), clock.Format(ate)
	sum := &costs.FinancialSummary{}

	// Débitos gerados no período (a coleção debt não tem soft-delete no schema).
	debts, err := r.app.FindRecordsByFilter("debt",
		"created_at >= {:from} && created_at < {:to}", "", 0, 0,
		dbx.Params{"from": deStr, "to": ateStr})
	if err != nil {
		return nil, mapError(err)
	}
	now := clock.Format(clock.Now())
	for _, d := range debts {
		amount := d.GetInt64("amount")
		sum.Generated += amount
		status := d.GetString("status")
		if status == billing.DebtAberto || status == billing.DebtParcial {
			sum.Open += amount
			due := d.GetString("due_date")
			if due != "" && due < now {
				sum.Overdue += amount
			}
		}
	}

	// Recebido no período (pagamentos).
	payments, err := r.app.FindRecordsByFilter("payment",
		"paid_at >= {:from} && paid_at < {:to}", "", 0, 0,
		dbx.Params{"from": deStr, "to": ateStr})
	if err != nil {
		return nil, mapError(err)
	}
	for _, p := range payments {
		sum.Received += p.GetInt64("amount")
	}
	return sum, nil
}

// CostTotalByKind soma os custos periodizados por categoria no intervalo. Soma
// todos os itens de custo ativos; o intervalo só determina o fator de
// periodização (paridade com o legado).
func (r *reportRepo) CostTotalByKind(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	items, err := r.app.FindRecordsByFilter("cost_item", notDeletedFilter, "", 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	days := ate.Sub(de).Hours() / 24
	if days < 1 {
		days = 1
	}
	kindByCategory := map[string]string{}
	out := map[string]int64{}
	for _, it := range items {
		categoryID := it.GetString("category_id")
		kind, ok := kindByCategory[categoryID]
		if !ok {
			cat, cerr := r.app.FindRecordById("cost_category", categoryID)
			if cerr != nil {
				if errors.Is(cerr, sql.ErrNoRows) {
					kind = ""
				} else {
					return nil, cerr
				}
			} else {
				kind = cat.GetString("kind")
			}
			kindByCategory[categoryID] = kind
		}
		item := itemFromRecord(it)
		out[kind] += costs.PeriodizedCost(item, days)
	}
	return out, nil
}

// SessionCostByPatient soma o custo atribuído às sessões de cada paciente no
// período (por starts_at da sessão).
func (r *reportRepo) SessionCostByPatient(ctx context.Context, de, ate time.Time) (map[string]int64, error) {
	sessions, err := r.app.FindRecordsByFilter("session",
		"starts_at >= {:from} && starts_at < {:to} && "+notDeletedFilter, "", 0, 0,
		dbx.Params{"from": clock.Format(de), "to": clock.Format(ate)})
	if err != nil {
		return nil, mapError(err)
	}
	patientBySession := map[string]string{}
	for _, s := range sessions {
		patientBySession[s.Id] = s.GetString("patient_id")
	}
	out := map[string]int64{}
	for sessionID, patientID := range patientBySession {
		sc, scErr := r.app.FindFirstRecordByFilter("session_cost", "session_id = {:s}", dbxParams("s", sessionID))
		if scErr != nil {
			if errors.Is(scErr, sql.ErrNoRows) {
				continue
			}
			return nil, scErr
		}
		out[patientID] += sc.GetInt64("amount")
	}
	return out, nil
}
