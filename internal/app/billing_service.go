package app

import (
	"context"
	"errors"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// BillingService gera débitos conforme o tipo de plano. É idempotente por
// construção (chave única) e recusa gerar débito para atendimento social. As
// regras de guarda/períodos/chaves vivem no domínio billing; o serviço
// orquestra plano vigente, idempotência e auditoria.
// Portado de c3ab585:internal/service/billing.go.
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
//
// O flag Bill == false é opt-out explícito ("esta sessão não gera cobrança").
func (s *BillingService) OnSessionFinished(ctx context.Context, sess *scheduling.Session) error {
	// Opt-out explícito: o terapeuta desmarcou a cobrança desta sessão.
	if !sess.Bill {
		return nil
	}
	plan, err := s.plans.GetActiveByPatient(ctx, sess.PatientID, sess.StartsAt)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Sem plano vigente: não há base para valor; não gera débito, mas
			// registra a condição de forma observável (R1.2) — sem conteúdo
			// clínico, só o fato de ter faltado o cadastro de plano.
			_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "session", sess.ID,
				map[string]any{"debt_skip_no_plan": true})
			return nil
		}
		return err
	}
	// Só planos por sessão geram débito aqui; social e fechado não.
	if !plan.GeneratesPerSessionDebt() {
		return nil
	}
	now := s.clock.Now()
	d := billing.NewSessionDebt(sess.PatientID, sess.ID, plan, now)
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
		period := billing.BillingPeriodFor(plan, ref)
		if period == "" {
			continue
		}
		// Gatilho por vencimento (R1.5): só cobra quando a data de referência
		// alcança o dia de vencimento do período. A idempotência por período
		// garante uma única cobrança por ciclo.
		if !billing.DueReached(plan, ref) {
			continue
		}
		now := s.clock.Now()
		d := billing.NewCycleDebt(plan, ref, now)
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

// ListDebts devolve todos os débitos.
func (s *BillingService) ListDebts(ctx context.Context) ([]*billing.Debt, error) {
	return s.debts.List(ctx)
}

// ListDebtsByPatient devolve os débitos de um paciente.
func (s *BillingService) ListDebtsByPatient(ctx context.Context, patientID string) ([]*billing.Debt, error) {
	return s.debts.ListByPatient(ctx, patientID)
}

// GetDebt devolve um débito por id.
func (s *BillingService) GetDebt(ctx context.Context, id string) (*billing.Debt, error) {
	return s.debts.Get(ctx, id)
}
