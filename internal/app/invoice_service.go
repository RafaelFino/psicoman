package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/pdf"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// InvoiceService gera o PDF de cobrança de um débito e o armazena no GED como
// lastro, vinculando-o ao débito (D9b / R9.3). NÃO há envio por email: o canal
// de envio (WhatsApp) é feito pela terapeuta, fora da plataforma — o Gmail saiu
// de escopo nesta arquitetura. Portado de c3ab585:internal/service/invoice.go
// (descartando SendCharge/Gmail e o cabeçalho de perfil do terapeuta, ainda não
// portado para o domínio rico).
type InvoiceService struct {
	debts    DebtRepository
	patients PatientRepository
	meta     GEDMetaRepository
	blobs    GEDBlobStore
	audit    *AuditService
	clock    clock.Clock
}

// NewInvoiceService cria o serviço de cobrança em PDF. audit é opcional (pode
// ser nil); o registro de auditoria é best-effort.
func NewInvoiceService(debts DebtRepository, patients PatientRepository, meta GEDMetaRepository, blobs GEDBlobStore, audit *AuditService) *InvoiceService {
	return &InvoiceService{
		debts:    debts,
		patients: patients,
		meta:     meta,
		blobs:    blobs,
		audit:    audit,
		clock:    clock.System{},
	}
}

// GeneratePDF gera (ou reaproveita) o PDF de cobrança do débito, armazena no GED
// (escopo do paciente, dedup por SHA-256) e vincula ao débito. Devolve o
// metadado do arquivo no GED. Idempotente por conteúdo: o mesmo débito gera o
// mesmo PDF, que é deduplicado por hash dentro do escopo do paciente.
func (s *InvoiceService) GeneratePDF(ctx context.Context, debtID string) (*domain.GEDFile, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, err
	}
	pat, err := s.patients.Get(ctx, debt.PatientID)
	if err != nil {
		return nil, err
	}

	pdfBytes := s.compose(debt, pat).Render()
	sum := sha256.Sum256(pdfBytes)
	sha := hex.EncodeToString(sum[:])

	// Dedup dentro do escopo do paciente: se já existe arquivo com este hash,
	// reaproveita (sem regravar os bytes nem duplicar o metadado).
	if existing, err := s.meta.FindByHash(ctx, debt.PatientID, sha); err == nil && existing != nil {
		if debt.PDFFileID != existing.ID {
			if err := s.debts.SetPDF(ctx, debt.ID, existing.ID); err != nil {
				return nil, err
			}
		}
		return existing, nil
	} else if err != nil && err != ErrNotFound {
		return nil, err
	}

	f := &domain.GEDFile{
		ID:        ulid.New(),
		PatientID: debt.PatientID,
		DebtID:    debt.ID,
		RelPath:   gedRelPath(debt.PatientID, sha),
		MIME:      "application/pdf",
		Size:      int64(len(pdfBytes)),
		SHA256:    sha,
		CreatedAt: s.clock.Now(),
	}
	if err := s.blobs.Put(ctx, f.RelPath, f.MIME, pdfBytes); err != nil {
		return nil, err
	}
	if err := s.meta.Insert(ctx, f); err != nil {
		return nil, err
	}
	if err := s.debts.SetPDF(ctx, debt.ID, f.ID); err != nil {
		return nil, err
	}
	if s.audit != nil {
		// Auditoria SEM conteúdo clínico nem financeiro sensível: só os ids.
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "debt", debt.ID,
			map[string]any{"ged_file_id": f.ID})
	}
	return f, nil
}

// GetPDF devolve os bytes do PDF de cobrança (gerando-o on-demand se ainda não
// existir). Usado pela rota de download.
func (s *InvoiceService) GetPDF(ctx context.Context, debtID string) ([]byte, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, err
	}
	var fileID string
	if debt.PDFFileID != "" {
		fileID = debt.PDFFileID
	} else {
		f, err := s.GeneratePDF(ctx, debtID)
		if err != nil {
			return nil, err
		}
		fileID = f.ID
	}
	meta, err := s.meta.Get(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return s.blobs.Get(ctx, meta.RelPath)
}

// gedRelPath compõe o caminho relativo (fileKey) do PDF de cobrança no storage,
// endereçado por paciente + hash do conteúdo. patientID vazio ("perfil do
// terapeuta") cai num diretório comum.
func gedRelPath(patientID, sha string) string {
	scope := patientID
	if scope == "" {
		scope = "_perfil"
	}
	return fmt.Sprintf("ged/%s/%s.pdf", scope, sha)
}

// compose monta o documento PDF da cobrança a partir do débito e do paciente.
// O cabeçalho de perfil do terapeuta (nome/CRP) fica de fora: o domínio rico
// ainda não porta o TherapistProfile (entra numa fase posterior).
func (s *InvoiceService) compose(debt *billing.Debt, pat *patient.Patient) *pdf.Document {
	doc := pdf.New("Cobrança de Atendimento")
	doc.AddLine("Paciente: " + pat.Name)
	if pat.CPF != "" {
		doc.AddLine("CPF: " + pat.CPF)
	}
	doc.AddLine("Email: " + pat.Email)
	doc.AddLine("")
	doc.AddLine("Documento nº: " + debt.ID)
	if debt.BillingPeriod != "" {
		doc.AddLine("Período de referência: " + debt.BillingPeriod)
	}
	if !debt.DueDate.IsZero() {
		doc.AddLine("Vencimento: " + debt.DueDate.In(clock.Location()).Format("02/01/2006"))
	}
	doc.AddLine("")
	doc.AddLine("Valor: " + formatBRL(debt.Amount))
	doc.AddLine("Situação: " + debtStatusPT(debt.Status))
	doc.AddLine("")
	doc.AddLine("Emitido em " + s.clock.Now().Format("02/01/2006 15:04"))
	return doc
}

// formatBRL formata centavos em "R$ 1.234,56" (pt-BR). Portado do legado.
func formatBRL(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	reais := cents / 100
	frac := cents % 100
	intStr := fmt.Sprintf("%d", reais)
	var grouped []byte
	for i, c := range []byte(intStr) {
		if i > 0 && (len(intStr)-i)%3 == 0 {
			grouped = append(grouped, '.')
		}
		grouped = append(grouped, c)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, string(grouped), frac)
}

// debtStatusPT traduz o status do débito para exibição no PDF.
func debtStatusPT(status string) string {
	switch status {
	case billing.DebtAberto:
		return "Em aberto"
	case billing.DebtPago:
		return "Pago"
	case billing.DebtParcial:
		return "Parcialmente pago"
	default:
		return status
	}
}
