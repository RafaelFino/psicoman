package billing

import (
	"time"

	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// BillingPeriodFor devolve o período de faturamento do plano na data ref:
// 'YYYY-MM' para fechado mensal, 'YYYY-Q' para fechado trimestral, "" caso o
// plano não gere ciclo. Portado de service/billing.go:billingPeriodFor.
func BillingPeriodFor(plan *Plan, ref time.Time) string {
	switch plan.Type {
	case PlanFechadoMensal:
		return MonthlyPeriod(ref)
	case PlanFechadoTrim:
		return QuarterlyPeriod(ref)
	default:
		return ""
	}
}

// DueDay devolve o dia de vencimento efetivo do plano (default 1 quando não
// configurado), limitado a 1–28. Portado de service/billing.go:dueDay.
func DueDay(plan *Plan) int {
	d := plan.DueDay
	if d < 1 {
		d = 1
	}
	if d > 28 {
		d = 28
	}
	return d
}

// CycleStartMonth devolve o ano/mês de início do período de ref: o próprio mês
// para mensal; o 1º mês do trimestre (jan/abr/jul/out) para trimestral. O
// vencimento é ancorado nesse mês. Portado de service/billing.go:cycleStartMonth.
func CycleStartMonth(plan *Plan, ref time.Time) (year int, month time.Month) {
	loc := clock.Location()
	r := ref.In(loc)
	if plan.Type == PlanFechadoTrim {
		// 1º mês do trimestre: jan/abr/jul/out.
		firstMonth := ((int(r.Month())-1)/3)*3 + 1
		return r.Year(), time.Month(firstMonth)
	}
	return r.Year(), r.Month()
}

// DueDateFor devolve a data real de vencimento do período (ano/mês do período +
// due_day), no fuso do projeto. Portado de service/billing.go:dueDateFor.
func DueDateFor(plan *Plan, ref time.Time) time.Time {
	year, month := CycleStartMonth(plan, ref)
	return time.Date(year, month, DueDay(plan), 0, 0, 0, 0, clock.Location())
}

// DueReached indica se a data de referência já alcançou o dia de vencimento do
// período (R1.5). Mensal: compara contra o due_day do próprio mês. Trimestral:
// o gatilho é o due_day do 1º mês do trimestre. Portado de
// service/billing.go:dueReached.
func DueReached(plan *Plan, ref time.Time) bool {
	r := ref.In(clock.Location())
	due := DueDateFor(plan, ref)
	// Mesma data ou depois do vencimento (comparando pelo dia).
	return !r.Before(due)
}

// NewSessionDebt monta o débito por sessão realizada de um plano por sessão,
// com idempotency_key por session_id e vencimento em now+7 dias. Paridade com
// service/billing.go:OnSessionFinished. O chamador (serviço de app) é
// responsável por só invocar quando plan.GeneratesPerSessionDebt() e bill=true.
func NewSessionDebt(patientID, sessionID string, plan *Plan, now time.Time) *Debt {
	return &Debt{
		ID:             ulid.New(),
		PatientID:      patientID,
		SessionID:      sessionID,
		PlanID:         plan.ID,
		Amount:         plan.Amount,
		DueDate:        now.AddDate(0, 0, 7),
		Status:         DebtAberto,
		IdempotencyKey: SessionIdempotencyKey(sessionID),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewCycleDebt monta o débito de fechamento de ciclo de um plano fechado, com
// idempotency_key por (plan_id + billing_period) e vencimento pela ancoragem do
// período. Paridade com service/billing.go:CloseCycles. O chamador deve só
// invocar quando plan.GeneratesCycleDebt(), período não vazio e DueReached.
func NewCycleDebt(plan *Plan, ref, now time.Time) *Debt {
	period := BillingPeriodFor(plan, ref)
	return &Debt{
		ID:             ulid.New(),
		PatientID:      plan.PatientID,
		PlanID:         plan.ID,
		BillingPeriod:  period,
		Amount:         plan.Amount,
		DueDate:        DueDateFor(plan, ref),
		Status:         DebtAberto,
		IdempotencyKey: CycleIdempotencyKey(plan.ID, period),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
