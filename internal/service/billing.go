package service

import (
	"context"
	"errors"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// DebtRepository persiste débitos, com geração idempotente por chave.
type DebtRepository interface {
	// InsertIfAbsent grava o débito só se a idempotency_key ainda não existir.
	// Devolve (created=true) quando inseriu; false quando já existia.
	InsertIfAbsent(ctx context.Context, d *domain.Debt) (bool, error)
	Get(ctx context.Context, id string) (*domain.Debt, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Debt, error)
	List(ctx context.Context) ([]*domain.Debt, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.Debt, error)
	UpdateStatus(ctx context.Context, id, status string) error
	SetPDF(ctx context.Context, id, pdfFileID string) error
}

// BillingService gera débitos conforme o tipo de plano. É idempotente por
// construção (chave única) e recusa gerar débito para atendimento social
// (docs/architecture.md §4.1, §4.1.1).
type BillingService struct {
	debts DebtRepository
	plans PlanRepository
	audit *AuditService
	clock clock.Clock
}

// NewBillingService cria o serviço de cobrança.
func NewBillingService(debts DebtRepository, plans PlanRepository, audit *AuditService) *BillingService {
	return &BillingService{debts: debts, plans: plans, audit: audit, clock: clock.System{}}
}

var _ SessionFinishedHook = (*BillingService)(nil)

// OnSessionFinished é o gatilho por sessão (planos por consulta/mês). Para
// planos fechados e social, não gera nada aqui. Idempotente por session_id.
func (s *BillingService) OnSessionFinished(ctx context.Context, sess *domain.Session) error {
	// Sem flag de cobrança, não gera débito.
	if !sess.Bill {
		return nil
	}
	plan, err := s.plans.GetActiveByPatient(ctx, sess.PatientID, sess.StartsAt)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Sem plano vigente: não há base para valor; não gera débito.
			return nil
		}
		return err
	}
	// Atendimento social nunca gera débito, mesmo com bill marcado.
	if plan.IsSocial() {
		return nil
	}
	// Planos fechados são cobrados por ciclo, não por sessão.
	if plan.IsFixedCycle() {
		return nil
	}
	// Planos por sessão: gera débito idempotente por session_id.
	now := s.clock.Now()
	d := &domain.Debt{
		ID:             ulid.New(),
		PatientID:      sess.PatientID,
		SessionID:      sess.ID,
		PlanID:         plan.ID,
		Amount:         plan.Amount,
		DueDate:        now.AddDate(0, 0, 7),
		Status:         domain.DebtAberto,
		IdempotencyKey: domain.SessionIdempotencyKey(sess.ID),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	created, err := s.debts.InsertIfAbsent(ctx, d)
	if err != nil {
		return err
	}
	if created {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "debt", d.ID,
			map[string]any{"session_id": sess.ID, "plan_type": plan.Type})
	}
	return nil
}

// CloseCycles é o job de fechamento de ciclo (cron diário): gera o débito do
// valor fixo dos planos fechados cujo período vigente iniciou em ref.
// Idempotente por (plan_id + billing_period). Devolve o nº de débitos criados.
func (s *BillingService) CloseCycles(ctx context.Context, ref time.Time) (int, error) {
	plans, err := s.plans.ListFixedCycleActive(ctx, ref)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, plan := range plans {
		period := billingPeriodFor(plan, ref)
		if period == "" {
			continue
		}
		now := s.clock.Now()
		d := &domain.Debt{
			ID:             ulid.New(),
			PatientID:      plan.PatientID,
			PlanID:         plan.ID,
			BillingPeriod:  period,
			Amount:         plan.Amount,
			DueDate:        now.AddDate(0, 0, 7),
			Status:         domain.DebtAberto,
			IdempotencyKey: domain.CycleIdempotencyKey(plan.ID, period),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		ok, err := s.debts.InsertIfAbsent(ctx, d)
		if err != nil {
			return created, err
		}
		if ok {
			created++
			_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "debt", d.ID,
				map[string]any{"plan_id": plan.ID, "billing_period": period, "plan_type": plan.Type})
		}
	}
	return created, nil
}

// billingPeriodFor devolve o período de faturamento do plano na data ref.
func billingPeriodFor(plan *domain.Plan, ref time.Time) string {
	switch plan.Type {
	case domain.PlanFechadoMensal:
		return domain.MonthlyPeriod(ref)
	case domain.PlanFechadoTrim:
		return domain.QuarterlyPeriod(ref)
	default:
		return ""
	}
}

// ListDebts devolve todos os débitos.
func (s *BillingService) ListDebts(ctx context.Context) ([]*domain.Debt, error) {
	return s.debts.List(ctx)
}

// ListDebtsByPatient devolve os débitos de um paciente.
func (s *BillingService) ListDebtsByPatient(ctx context.Context, patientID string) ([]*domain.Debt, error) {
	return s.debts.ListByPatient(ctx, patientID)
}

// GetDebt devolve um débito por id.
func (s *BillingService) GetDebt(ctx context.Context, id string) (*domain.Debt, error) {
	return s.debts.Get(ctx, id)
}
