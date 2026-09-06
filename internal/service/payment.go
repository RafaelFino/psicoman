package service

import (
	"bytes"
	"context"
	"io"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// PaymentRepository persiste pagamentos.
type PaymentRepository interface {
	Insert(ctx context.Context, p *domain.Payment) error
	ListByDebt(ctx context.Context, debtID string) ([]*domain.Payment, error)
	// SumByDebt devolve o total já pago de um débito.
	SumByDebt(ctx context.Context, debtID string) (int64, error)
}

// PaymentService registra pagamentos, atualiza o status do débito conforme o
// total quitado e anexa comprovantes ao GED (requirements §3.4).
type PaymentService struct {
	payments PaymentRepository
	debts    DebtRepository
	ged      *GEDService
	audit    *AuditService
	clock    clock.Clock
}

// NewPaymentService cria o serviço de pagamentos.
func NewPaymentService(payments PaymentRepository, debts DebtRepository, ged *GEDService, audit *AuditService) *PaymentService {
	return &PaymentService{payments: payments, debts: debts, ged: ged, audit: audit, clock: clock.System{}}
}

// PayInput são os dados de um pagamento.
type PayInput struct {
	Amount int64
	Method string
}

// Pay registra um pagamento e recalcula o status do débito (aberto → parcial →
// pago) conforme o total quitado. Devolve o débito atualizado.
func (s *PaymentService) Pay(ctx context.Context, debtID string, in PayInput) (*domain.Debt, *domain.Payment, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, nil, err
	}
	if in.Amount <= 0 {
		return nil, nil, NewValidation("O valor do pagamento deve ser maior que zero.")
	}

	now := s.clock.Now()
	pay := &domain.Payment{
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
	status := domain.DebtParcial
	if paid >= debt.Amount {
		status = domain.DebtPago
	}
	if err := s.debts.UpdateStatus(ctx, debtID, status); err != nil {
		return nil, nil, err
	}
	debt.Status = status

	_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtPay, "debt", debtID,
		map[string]any{"payment_id": pay.ID, "amount": in.Amount, "status": status})
	return debt, pay, nil
}

// AttachReceipt anexa um comprovante de pagamento ao GED, vinculado ao pagamento.
func (s *PaymentService) AttachReceipt(ctx context.Context, debtID, paymentID, mime string, content io.Reader) (*domain.GEDFile, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, err
	}
	return s.ged.Store(ctx, GEDLink{PatientID: debt.PatientID, DebtID: debtID, PaymentID: paymentID},
		mime, bytes.NewReader(data))
}

// ListPayments devolve os pagamentos de um débito.
func (s *PaymentService) ListPayments(ctx context.Context, debtID string) ([]*domain.Payment, error) {
	return s.payments.ListByDebt(ctx, debtID)
}
