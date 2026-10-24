package app

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PaymentService registra pagamentos e atualiza o status do débito conforme o
// total quitado. A regra de status (aberto→parcial→pago) vive no agregado
// billing.Debt.ApplyPayment; o serviço coordena o I/O de somar pagamentos.
// Portado de c3ab585:internal/service/payment.go.
type PaymentService struct {
	payments PaymentRepository
	debts    DebtRepository
	audit    *AuditService
	clock    clock.Clock
}

// NewPaymentService cria o serviço de pagamentos.
func NewPaymentService(payments PaymentRepository, debts DebtRepository, audit *AuditService) *PaymentService {
	return &PaymentService{payments: payments, debts: debts, audit: audit, clock: clock.System{}}
}

// PayInput são os dados de um pagamento.
type PayInput struct {
	Amount int64
	Method string
}

// Pay registra um pagamento e recalcula o status do débito (aberto → parcial →
// pago) conforme o total quitado. Devolve o débito atualizado e o pagamento.
func (s *PaymentService) Pay(ctx context.Context, debtID string, in PayInput) (*billing.Debt, *billing.Payment, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, nil, err
	}
	if in.Amount <= 0 {
		return nil, nil, NewValidation("O valor do pagamento deve ser maior que zero.")
	}
	now := s.clock.Now()
	pay := &billing.Payment{
		ID: ulid.New(), DebtID: debtID, Amount: in.Amount,
		PaidAt: now, Method: in.Method, CreatedAt: now,
	}
	if err := s.payments.Insert(ctx, pay); err != nil {
		return nil, nil, err
	}
	paid, err := s.payments.SumByDebt(ctx, debtID)
	if err != nil {
		return nil, nil, err
	}
	status := debt.ApplyPayment(paid)
	if err := s.debts.UpdateStatus(ctx, debtID, status); err != nil {
		return nil, nil, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtPay, "debt", debtID,
			map[string]any{"payment_id": pay.ID, "amount": in.Amount, "status": status})
	}
	return debt, pay, nil
}

// ListPayments devolve os pagamentos de um débito.
func (s *PaymentService) ListPayments(ctx context.Context, debtID string) ([]*billing.Payment, error) {
	return s.payments.ListByDebt(ctx, debtID)
}
