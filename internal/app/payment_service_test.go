package app

import (
	"context"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func newPayment(payments PaymentRepository, debts DebtRepository, c clock.Clock) *PaymentService {
	audit, _ := newAudit()
	audit.clock = c
	s := NewPaymentService(payments, debts, audit)
	s.clock = c
	return s
}

func TestPaymentPay_ParcialDepoisTotal(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{ID: "d1", PatientID: "p1", Amount: 10000, Status: billing.DebtAberto}
	payments := newPaymentRepo()
	svc := newPayment(payments, debts, fixedAt(2026, 3, 10))

	// Pagamento parcial.
	d, _, err := svc.Pay(ctx, "d1", PayInput{Amount: 4000, Method: "pix"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != billing.DebtParcial {
		t.Fatalf("esperava parcial, veio %s", d.Status)
	}
	if debts.byID["d1"].Status != billing.DebtParcial {
		t.Fatalf("status persistido divergente: %s", debts.byID["d1"].Status)
	}

	// Pagamento que completa o total.
	d, _, err = svc.Pay(ctx, "d1", PayInput{Amount: 6000, Method: "pix"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != billing.DebtPago {
		t.Fatalf("esperava pago, veio %s", d.Status)
	}
	if debts.byID["d1"].Status != billing.DebtPago {
		t.Fatalf("status persistido divergente: %s", debts.byID["d1"].Status)
	}
}

func TestPaymentPay_AmountInvalido(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{ID: "d1", PatientID: "p1", Amount: 10000, Status: billing.DebtAberto}
	svc := newPayment(newPaymentRepo(), debts, fixedAt(2026, 3, 10))

	_, _, err := svc.Pay(ctx, "d1", PayInput{Amount: 0})
	if err == nil {
		t.Fatal("esperava erro de validação para valor <= 0")
	}
}
