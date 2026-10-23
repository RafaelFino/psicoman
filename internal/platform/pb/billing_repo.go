package pb

import (
	"context"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// planRepo implementa app.PlanRepository sobre a coleção plan.
type planRepo struct{ app core.App }

var _ app.PlanRepository = (*planRepo)(nil)

// NewPlanRepository cria o repositório de planos.
func NewPlanRepository(a core.App) app.PlanRepository { return &planRepo{app: a} }

func (r *planRepo) toRecord(rec *core.Record, p *billing.Plan) {
	rec.Set("id", p.ID)
	rec.Set("patient_id", p.PatientID)
	rec.Set("contract_id", p.ContractID)
	rec.Set("type", p.Type)
	rec.Set("amount", p.Amount)
	rec.Set("due_day", p.DueDay)
	rec.Set("starts_at", formatTime(p.StartsAt))
	rec.Set("ends_at", formatTime(p.EndsAt))
	rec.Set("created_at", formatTime(p.CreatedAt))
	rec.Set("updated_at", formatTime(p.UpdatedAt))
}

func planFromRecord(rec *core.Record) *billing.Plan {
	return &billing.Plan{
		ID:         rec.Id,
		PatientID:  rec.GetString("patient_id"),
		ContractID: rec.GetString("contract_id"),
		Type:       rec.GetString("type"),
		Amount:     rec.GetInt64("amount"),
		DueDay:     rec.GetInt("due_day"),
		StartsAt:   parseTime(rec.GetString("starts_at")),
		EndsAt:     parseTime(rec.GetString("ends_at")),
		CreatedAt:  parseTime(rec.GetString("created_at")),
		UpdatedAt:  parseTime(rec.GetString("updated_at")),
	}
}

func (r *planRepo) Create(ctx context.Context, p *billing.Plan) error {
	col, err := r.app.FindCollectionByNameOrId("plan")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *planRepo) Update(ctx context.Context, p *billing.Plan) error {
	rec, err := getRecordByID(r.app, "plan", p.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *planRepo) Get(ctx context.Context, id string) (*billing.Plan, error) {
	rec, err := r.app.FindFirstRecordByFilter("plan", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return planFromRecord(rec), nil
}

// GetActiveByPatient devolve o plano vigente do paciente em ref: vigência iniciada
// (starts_at <= ref) e sem fim ou com fim após ref. Mais recente primeiro.
func (r *planRepo) GetActiveByPatient(ctx context.Context, patientID string, ref time.Time) (*billing.Plan, error) {
	recs, err := r.listByFilter(
		"patient_id = {:p} && starts_at <= {:ref} && (ends_at = '' || ends_at >= {:ref}) && "+notDeletedFilter,
		"-starts_at",
		dbx.Params{"p": patientID, "ref": clock.Format(ref)})
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, app.ErrNotFound
	}
	return recs[0], nil
}

func (r *planRepo) ListByPatient(ctx context.Context, patientID string) ([]*billing.Plan, error) {
	return r.listByFilter("patient_id = {:p} && "+notDeletedFilter, "-starts_at", dbxParams("p", patientID))
}

// ListFixedCycleActive devolve planos fechados (mensal/trimestral) vigentes em ref.
func (r *planRepo) ListFixedCycleActive(ctx context.Context, ref time.Time) ([]*billing.Plan, error) {
	return r.listByFilter(
		"(type = {:m} || type = {:t}) && starts_at <= {:ref} && (ends_at = '' || ends_at >= {:ref}) && "+notDeletedFilter,
		"starts_at",
		dbx.Params{
			"m":   billing.PlanFechadoMensal,
			"t":   billing.PlanFechadoTrim,
			"ref": clock.Format(ref),
		})
}

func (r *planRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "plan", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *planRepo) listByFilter(filter, sort string, params dbx.Params) ([]*billing.Plan, error) {
	recs, err := r.app.FindRecordsByFilter("plan", filter, sort, 0, 0, params)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*billing.Plan, 0, len(recs))
	for _, rec := range recs {
		out = append(out, planFromRecord(rec))
	}
	return out, nil
}

// contractRepo implementa app.ContractRepository sobre a coleção contract.
type contractRepo struct{ app core.App }

var _ app.ContractRepository = (*contractRepo)(nil)

// NewContractRepository cria o repositório de contratos.
func NewContractRepository(a core.App) app.ContractRepository { return &contractRepo{app: a} }

func (r *contractRepo) toRecord(rec *core.Record, c *billing.Contract) {
	rec.Set("id", c.ID)
	rec.Set("patient_id", c.PatientID)
	rec.Set("plan_id", c.PlanID)
	rec.Set("starts_at", formatTime(c.StartsAt))
	rec.Set("ends_at", formatTime(c.EndsAt))
	rec.Set("adjustment_rule", c.AdjustmentRule)
	rec.Set("artifact_file_id", c.ArtifactFileID)
	rec.Set("created_at", formatTime(c.CreatedAt))
	rec.Set("updated_at", formatTime(c.UpdatedAt))
}

func contractFromRecord(rec *core.Record) *billing.Contract {
	return &billing.Contract{
		ID:             rec.Id,
		PatientID:      rec.GetString("patient_id"),
		PlanID:         rec.GetString("plan_id"),
		StartsAt:       parseTime(rec.GetString("starts_at")),
		EndsAt:         parseTime(rec.GetString("ends_at")),
		AdjustmentRule: rec.GetString("adjustment_rule"),
		ArtifactFileID: rec.GetString("artifact_file_id"),
		CreatedAt:      parseTime(rec.GetString("created_at")),
		UpdatedAt:      parseTime(rec.GetString("updated_at")),
	}
}

func (r *contractRepo) Create(ctx context.Context, c *billing.Contract) error {
	col, err := r.app.FindCollectionByNameOrId("contract")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, c)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *contractRepo) Update(ctx context.Context, c *billing.Contract) error {
	rec, err := getRecordByID(r.app, "contract", c.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, c)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *contractRepo) Get(ctx context.Context, id string) (*billing.Contract, error) {
	rec, err := r.app.FindFirstRecordByFilter("contract", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return contractFromRecord(rec), nil
}

func (r *contractRepo) ListByPatient(ctx context.Context, patientID string) ([]*billing.Contract, error) {
	recs, err := r.app.FindRecordsByFilter("contract", "patient_id = {:p} && "+notDeletedFilter, "-starts_at", 0, 0, dbxParams("p", patientID))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*billing.Contract, 0, len(recs))
	for _, rec := range recs {
		out = append(out, contractFromRecord(rec))
	}
	return out, nil
}

func (r *contractRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "contract", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// debtRepo implementa app.DebtRepository sobre a coleção debt.
type debtRepo struct{ app core.App }

var _ app.DebtRepository = (*debtRepo)(nil)

// NewDebtRepository cria o repositório de débitos.
func NewDebtRepository(a core.App) app.DebtRepository { return &debtRepo{app: a} }

func (r *debtRepo) toRecord(rec *core.Record, d *billing.Debt) {
	rec.Set("id", d.ID)
	rec.Set("patient_id", d.PatientID)
	rec.Set("session_id", d.SessionID)
	rec.Set("plan_id", d.PlanID)
	rec.Set("billing_period", d.BillingPeriod)
	rec.Set("amount", d.Amount)
	rec.Set("due_date", formatTime(d.DueDate))
	rec.Set("status", d.Status)
	rec.Set("idempotency_key", d.IdempotencyKey)
	rec.Set("pdf_file_id", d.PDFFileID)
	rec.Set("created_at", formatTime(d.CreatedAt))
	rec.Set("updated_at", formatTime(d.UpdatedAt))
}

func debtFromRecord(rec *core.Record) *billing.Debt {
	return &billing.Debt{
		ID:             rec.Id,
		PatientID:      rec.GetString("patient_id"),
		SessionID:      rec.GetString("session_id"),
		PlanID:         rec.GetString("plan_id"),
		BillingPeriod:  rec.GetString("billing_period"),
		Amount:         rec.GetInt64("amount"),
		DueDate:        parseTime(rec.GetString("due_date")),
		Status:         rec.GetString("status"),
		IdempotencyKey: rec.GetString("idempotency_key"),
		PDFFileID:      rec.GetString("pdf_file_id"),
		CreatedAt:      parseTime(rec.GetString("created_at")),
		UpdatedAt:      parseTime(rec.GetString("updated_at")),
	}
}

// InsertIfAbsent grava o débito só se a idempotency_key ainda não existir.
// Checa a existência antes de inserir (o UNIQUE ux_debt_idempotency é a rede de
// segurança): se já existe, devolve created=false sem gravar; se a gravação
// colide com o UNIQUE por corrida, também traduz para created=false.
func (r *debtRepo) InsertIfAbsent(ctx context.Context, d *billing.Debt) (bool, error) {
	existing, err := r.GetByIdempotencyKey(ctx, d.IdempotencyKey)
	if err != nil && !errors.Is(err, app.ErrNotFound) {
		return false, err
	}
	if existing != nil {
		return false, nil
	}
	col, err := r.app.FindCollectionByNameOrId("debt")
	if err != nil {
		return false, mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, d)
	if err := r.app.SaveWithContext(ctx, rec); err != nil {
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *debtRepo) Get(ctx context.Context, id string) (*billing.Debt, error) {
	rec, err := getRecordByID(r.app, "debt", id)
	if err != nil {
		return nil, err
	}
	return debtFromRecord(rec), nil
}

func (r *debtRepo) GetByIdempotencyKey(ctx context.Context, key string) (*billing.Debt, error) {
	rec, err := r.app.FindFirstRecordByFilter("debt", "idempotency_key = {:k}", dbxParams("k", key))
	if err != nil {
		return nil, mapError(err)
	}
	return debtFromRecord(rec), nil
}

func (r *debtRepo) List(ctx context.Context) ([]*billing.Debt, error) {
	return r.listByFilter("", "-created_at", nil)
}

func (r *debtRepo) ListByPatient(ctx context.Context, patientID string) ([]*billing.Debt, error) {
	return r.listByFilter("patient_id = {:p}", "-created_at", dbxParams("p", patientID))
}

func (r *debtRepo) ListBySessionIDs(ctx context.Context, sessionIDs []string) ([]*billing.Debt, error) {
	out := make([]*billing.Debt, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		if sid == "" {
			continue
		}
		recs, err := r.listByFilter("session_id = {:s}", "-created_at", dbxParams("s", sid))
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	return out, nil
}

func (r *debtRepo) ListByCreatedRange(ctx context.Context, from, to time.Time) ([]*billing.Debt, error) {
	return r.listByFilter(
		"created_at >= {:from} && created_at < {:to}",
		"-created_at",
		dbx.Params{"from": clock.Format(from), "to": clock.Format(to)})
}

func (r *debtRepo) ListDueBetween(ctx context.Context, from, to time.Time) ([]*billing.Debt, error) {
	return r.listByFilter(
		"(status = {:a} || status = {:p}) && due_date >= {:from} && due_date < {:to}",
		"due_date",
		dbx.Params{
			"a":    billing.DebtAberto,
			"p":    billing.DebtParcial,
			"from": clock.Format(from),
			"to":   clock.Format(to),
		})
}

func (r *debtRepo) UpdateStatus(ctx context.Context, id, status string) error {
	rec, err := getRecordByID(r.app, "debt", id)
	if err != nil {
		return err
	}
	rec.Set("status", status)
	rec.Set("updated_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *debtRepo) SetPDF(ctx context.Context, id, pdfFileID string) error {
	rec, err := getRecordByID(r.app, "debt", id)
	if err != nil {
		return err
	}
	rec.Set("pdf_file_id", pdfFileID)
	rec.Set("updated_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *debtRepo) listByFilter(filter, sort string, params dbx.Params) ([]*billing.Debt, error) {
	var recs []*core.Record
	var err error
	if params == nil {
		recs, err = r.app.FindRecordsByFilter("debt", filter, sort, 0, 0)
	} else {
		recs, err = r.app.FindRecordsByFilter("debt", filter, sort, 0, 0, params)
	}
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*billing.Debt, 0, len(recs))
	for _, rec := range recs {
		out = append(out, debtFromRecord(rec))
	}
	return out, nil
}

// paymentRepo implementa app.PaymentRepository sobre a coleção payment.
type paymentRepo struct{ app core.App }

var _ app.PaymentRepository = (*paymentRepo)(nil)

// NewPaymentRepository cria o repositório de pagamentos.
func NewPaymentRepository(a core.App) app.PaymentRepository { return &paymentRepo{app: a} }

func (r *paymentRepo) toRecord(rec *core.Record, p *billing.Payment) {
	rec.Set("id", p.ID)
	rec.Set("debt_id", p.DebtID)
	rec.Set("amount", p.Amount)
	rec.Set("paid_at", formatTime(p.PaidAt))
	rec.Set("method", p.Method)
	rec.Set("created_at", formatTime(p.CreatedAt))
}

func paymentFromRecord(rec *core.Record) *billing.Payment {
	return &billing.Payment{
		ID:        rec.Id,
		DebtID:    rec.GetString("debt_id"),
		Amount:    rec.GetInt64("amount"),
		PaidAt:    parseTime(rec.GetString("paid_at")),
		Method:    rec.GetString("method"),
		CreatedAt: parseTime(rec.GetString("created_at")),
	}
}

func (r *paymentRepo) Insert(ctx context.Context, p *billing.Payment) error {
	col, err := r.app.FindCollectionByNameOrId("payment")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, p)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *paymentRepo) ListByDebt(ctx context.Context, debtID string) ([]*billing.Payment, error) {
	recs, err := r.app.FindRecordsByFilter("payment", "debt_id = {:d}", "paid_at", 0, 0, dbxParams("d", debtID))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*billing.Payment, 0, len(recs))
	for _, rec := range recs {
		out = append(out, paymentFromRecord(rec))
	}
	return out, nil
}

// SumByDebt devolve o total já pago de um débito (soma em Go sobre os pagamentos).
func (r *paymentRepo) SumByDebt(ctx context.Context, debtID string) (int64, error) {
	payments, err := r.ListByDebt(ctx, debtID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, p := range payments {
		total += p.Amount
	}
	return total, nil
}

func (r *paymentRepo) ListByRange(ctx context.Context, from, to time.Time) ([]*billing.Payment, error) {
	recs, err := r.app.FindRecordsByFilter("payment",
		"paid_at >= {:from} && paid_at < {:to}", "paid_at", 0, 0,
		dbx.Params{"from": clock.Format(from), "to": clock.Format(to)})
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*billing.Payment, 0, len(recs))
	for _, rec := range recs {
		out = append(out, paymentFromRecord(rec))
	}
	return out, nil
}
