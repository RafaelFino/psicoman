package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/pdf"
)

// InvoiceService gera o PDF de cobrança de um débito e o armazena no GED como
// lastro, vinculando-o ao débito (requirements §3.4).
type InvoiceService struct {
	debts     DebtRepository
	patients  PatientRepository
	ged       *GEDService
	therapist *TherapistService
	audit     *AuditService
	gmail     GmailClient
}

// NewInvoiceService cria o serviço de cobrança em PDF.
func NewInvoiceService(debts DebtRepository, patients PatientRepository, ged *GEDService, therapist *TherapistService, audit *AuditService) *InvoiceService {
	return &InvoiceService{debts: debts, patients: patients, ged: ged, therapist: therapist, audit: audit}
}

// SetGmail injeta o cliente Gmail para envio de cobranças (Task 16).
func (s *InvoiceService) SetGmail(gmail GmailClient) { s.gmail = gmail }

// SendCharge envia a cobrança por email ao paciente (best-effort): gera o PDF
// (lastro no GED) e envia um email HTML com o resumo. Falha de envio NÃO
// bloqueia — o PDF permanece disponível (psicoman-google-api.md).
func (s *InvoiceService) SendCharge(ctx context.Context, debtID string) error {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return err
	}
	patient, err := s.patients.Get(ctx, debt.PatientID)
	if err != nil {
		return err
	}
	// Garante o PDF como lastro (idempotente na prática pela dedup do GED).
	if debt.PDFFileID == "" {
		if _, err := s.GeneratePDF(ctx, debtID); err != nil {
			return err
		}
	}
	if s.gmail == nil {
		return NewValidation("Envio de email não configurado.")
	}
	html := chargeEmailHTML(patient.Name, debt)
	// Best-effort: erro de envio é ignorado pelo caller de negócio.
	_ = s.gmail.Send(ctx, patient.Email, "Cobrança de atendimento", html)
	_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "debt_email", debtID,
		map[string]any{"to": patient.Email})
	return nil
}

// GeneratePDF gera (ou regenera) o PDF de cobrança do débito, armazena no GED
// (escopo do paciente) e vincula ao débito. Devolve o arquivo do GED.
func (s *InvoiceService) GeneratePDF(ctx context.Context, debtID string) (*domain.GEDFile, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, err
	}
	patient, err := s.patients.Get(ctx, debt.PatientID)
	if err != nil {
		return nil, err
	}

	doc := s.compose(ctx, debt, patient)
	pdfBytes := doc.Render()

	f, err := s.ged.Store(ctx, GEDLink{PatientID: debt.PatientID, DebtID: debt.ID}, "application/pdf", bytes.NewReader(pdfBytes))
	if err != nil {
		return nil, err
	}
	if err := s.debts.SetPDF(ctx, debt.ID, f.ID); err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, "sistema", domain.AuditActionDebtGenerate, "debt_pdf", debt.ID,
		map[string]any{"ged_file_id": f.ID})
	return f, nil
}

// GetPDF devolve o conteúdo do PDF de cobrança já gerado (ou o gera on-demand).
func (s *InvoiceService) GetPDF(ctx context.Context, debtID string) ([]byte, error) {
	debt, err := s.debts.Get(ctx, debtID)
	if err != nil {
		return nil, err
	}
	if debt.PDFFileID == "" {
		f, err := s.GeneratePDF(ctx, debtID)
		if err != nil {
			return nil, err
		}
		debt.PDFFileID = f.ID
	}
	_, data, err := s.ged.Read(ctx, debt.PDFFileID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// compose monta o documento PDF da cobrança.
func (s *InvoiceService) compose(ctx context.Context, debt *domain.Debt, patient *domain.Patient) *pdf.Document {
	doc := pdf.New("Cobrança de Atendimento")

	// Cabeçalho do terapeuta (se configurado).
	if prof, err := s.therapist.GetProfile(ctx); err == nil {
		doc.AddLine("Profissional: " + prof.Name)
		if prof.CRP != "" {
			doc.AddLine("CRP: " + prof.CRP)
		}
		doc.AddLine("")
	}

	doc.AddLine("Paciente: " + patient.Name)
	if patient.CPF != "" {
		doc.AddLine("CPF: " + patient.CPF)
	}
	doc.AddLine("Email: " + patient.Email)
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
	doc.AddLine("Situação: " + statusPT(debt.Status))
	doc.AddLine("")
	doc.AddLine("Emitido em " + clock.Now().Format("02/01/2006 15:04"))

	return doc
}

// formatBRL formata centavos em "R$ 1.234,56".
func formatBRL(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	reais := cents / 100
	frac := cents % 100
	// Agrupamento de milhar.
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

// chargeEmailHTML monta o corpo HTML do email de cobrança.
func chargeEmailHTML(patientName string, debt *domain.Debt) string {
	due := ""
	if !debt.DueDate.IsZero() {
		due = "<p>Vencimento: " + debt.DueDate.In(clock.Location()).Format("02/01/2006") + "</p>"
	}
	return "<div>" +
		"<p>Olá, " + patientName + ".</p>" +
		"<p>Segue a cobrança referente ao seu atendimento.</p>" +
		"<p><strong>Valor: " + formatBRL(debt.Amount) + "</strong></p>" +
		due +
		"<p>Situação: " + statusPT(debt.Status) + "</p>" +
		"<p>Documento nº " + debt.ID + "</p>" +
		"</div>"
}

func statusPT(status string) string {
	switch status {
	case domain.DebtAberto:
		return "Em aberto"
	case domain.DebtPago:
		return "Pago"
	case domain.DebtParcial:
		return "Parcialmente pago"
	default:
		return status
	}
}
