package billing

import (
	"fmt"
	"time"
)

// Status de débito.
const (
	DebtAberto  = "aberto"
	DebtPago    = "pago"
	DebtParcial = "parcial"
)

// Debt é uma entrada de valor a receber.
type Debt struct {
	ID             string
	PatientID      string
	SessionID      string // nulo para débitos de plano fechado
	PlanID         string
	BillingPeriod  string // 'YYYY-MM' ou 'YYYY-Q' quando aplicável
	Amount         int64  // centavos
	DueDate        time.Time
	Status         string
	IdempotencyKey string
	PDFFileID      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Payment é um pagamento que quita (total ou parcialmente) um débito.
type Payment struct {
	ID        string
	DebtID    string
	Amount    int64 // centavos
	PaidAt    time.Time
	Method    string
	CreatedAt time.Time
}

// SessionIdempotencyKey compõe a chave de idempotência de débito por sessão.
func SessionIdempotencyKey(sessionID string) string {
	return "session:" + sessionID
}

// CycleIdempotencyKey compõe a chave de idempotência de débito por ciclo.
func CycleIdempotencyKey(planID, billingPeriod string) string {
	return fmt.Sprintf("cycle:%s:%s", planID, billingPeriod)
}

// MonthlyPeriod devolve o período de faturamento mensal 'YYYY-MM' para ref.
func MonthlyPeriod(ref time.Time) string {
	return ref.Format("2006-01")
}

// QuarterlyPeriod devolve o período trimestral 'YYYY-Q' (Q em 1..4) para ref.
func QuarterlyPeriod(ref time.Time) string {
	q := (int(ref.Month())-1)/3 + 1
	return fmt.Sprintf("%d-Q%d", ref.Year(), q)
}

// ApplyPayment recalcula e aplica o status do débito a partir do total já pago
// (soma de pagamentos, em centavos). Devolve o novo status: pago quando o total
// alcança o valor do débito, parcial caso contrário. Extrai a regra do legado
// PaymentService.Pay (domínio rico); o I/O de somar pagamentos fica no serviço.
func (d *Debt) ApplyPayment(totalPaid int64) string {
	status := DebtParcial
	if totalPaid >= d.Amount {
		status = DebtPago
	}
	d.Status = status
	return status
}
