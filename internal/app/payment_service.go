package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

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
	meta     GEDMetaRepository // opcional: comprovante de pagamento (R1.9)
	blobs    GEDBlobStore      // opcional: bytes do comprovante no GED
	clock    clock.Clock
	metrics  Metrics
}

// NewPaymentService cria o serviço de pagamentos.
func NewPaymentService(payments PaymentRepository, debts DebtRepository, audit *AuditService) *PaymentService {
	return &PaymentService{payments: payments, debts: debts, audit: audit, clock: clock.System{}, metrics: NopMetrics{}}
}

// SetMetrics injeta a porta de observabilidade (default NopMetrics). Os eventos
// são contados SÓ no caminho de sucesso e nunca bloqueiam a operação.
func (s *PaymentService) SetMetrics(m Metrics) {
	if m == nil {
		m = NopMetrics{}
	}
	s.metrics = m
}

// SetGED injeta as portas do GED para anexar comprovantes de pagamento (R1.9).
// Opcional: sem elas, um pagamento com comprovante é rejeitado com validação.
func (s *PaymentService) SetGED(meta GEDMetaRepository, blobs GEDBlobStore) {
	s.meta = meta
	s.blobs = blobs
}

// PayInput são os dados de um pagamento. Receipt* são o comprovante OPCIONAL
// (bytes já decodificados + mime); quando presentes, são gravados no GED
// vinculados ao pagamento.
type PayInput struct {
	Amount  int64
	Method  string
	Receipt []byte // comprovante opcional (vazio = sem anexo)
	MIME    string
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
	// Comprovante opcional no GED (R1.9), vinculado ao pagamento + paciente.
	if len(in.Receipt) > 0 {
		if err := s.storeReceipt(ctx, debt, pay, in.Receipt, in.MIME); err != nil {
			return nil, nil, err
		}
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
	s.metrics.PaymentRecorded()
	return debt, pay, nil
}

// ListPayments devolve os pagamentos de um débito.
func (s *PaymentService) ListPayments(ctx context.Context, debtID string) ([]*billing.Payment, error) {
	return s.payments.ListByDebt(ctx, debtID)
}

// storeReceipt grava o comprovante de pagamento no GED (dedup por SHA-256 no
// escopo do paciente), vinculado ao pagamento e ao débito. Mesma disciplina do
// InvoiceService: bytes no blob store, metadado no GEDMetaRepository. Rejeita
// com validação se as portas do GED não foram injetadas (SetGED).
func (s *PaymentService) storeReceipt(ctx context.Context, debt *billing.Debt, pay *billing.Payment, content []byte, mime string) error {
	if s.meta == nil || s.blobs == nil {
		return NewValidation("Anexo de comprovante indisponível: GED não configurado.")
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])

	// Dedup no escopo do paciente: reaproveita se o mesmo arquivo já existe.
	if existing, err := s.meta.FindByHash(ctx, debt.PatientID, sha); err == nil && existing != nil {
		return nil
	} else if err != nil && err != ErrNotFound {
		return err
	}

	f := &domain.GEDFile{
		ID:        ulid.New(),
		PatientID: debt.PatientID,
		DebtID:    debt.ID,
		PaymentID: pay.ID,
		RelPath:   gedRelPath(debt.PatientID, sha),
		MIME:      mime,
		Size:      int64(len(content)),
		SHA256:    sha,
		CreatedAt: s.clock.Now(),
	}
	if err := s.blobs.Put(ctx, f.RelPath, f.MIME, content); err != nil {
		return err
	}
	return s.meta.Insert(ctx, f)
}
