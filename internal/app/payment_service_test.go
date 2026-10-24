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

// TestPaymentComComprovante grava o comprovante no GED, vinculado ao pagamento
// e ao paciente (R1.9).
func TestPaymentComComprovante(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{ID: "d1", PatientID: "p1", Amount: 10000, Status: billing.DebtAberto}
	payments := newPaymentRepo()
	meta := newGEDMetaRepo()
	blob := newGEDBlob()
	svc := newPayment(payments, debts, fixedAt(2026, 3, 10))
	svc.SetGED(meta, blob)

	comprovante := []byte("%PDF-1.4 comprovante de pix")
	_, pay, err := svc.Pay(ctx, "d1", PayInput{Amount: 10000, Method: "pix", Receipt: comprovante, MIME: "application/pdf"})
	if err != nil {
		t.Fatalf("Pay com comprovante: %v", err)
	}

	// O metadado foi gravado no escopo do paciente, vinculado ao pagamento.
	files, _ := meta.ListByPatient(ctx, "p1")
	if len(files) != 1 {
		t.Fatalf("esperava 1 comprovante no GED, veio %d", len(files))
	}
	if files[0].PaymentID != pay.ID || files[0].DebtID != "d1" {
		t.Errorf("comprovante não vinculado ao pagamento/débito: %+v", files[0])
	}
	if blob.puts != 1 {
		t.Errorf("bytes do comprovante não gravados: puts=%d", blob.puts)
	}
}

// TestPaymentComprovanteSemGED422 rejeita comprovante quando o GED não foi
// injetado (SetGED).
func TestPaymentComprovanteSemGED422(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{ID: "d1", PatientID: "p1", Amount: 10000, Status: billing.DebtAberto}
	svc := newPayment(newPaymentRepo(), debts, fixedAt(2026, 3, 10))
	// Sem SetGED.
	if _, _, err := svc.Pay(ctx, "d1", PayInput{Amount: 10000, Receipt: []byte("x")}); err == nil {
		t.Error("esperava erro de validação sem GED configurado")
	}
}
