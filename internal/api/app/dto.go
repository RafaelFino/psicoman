// Package app reúne os handlers do ATOR terapeuta (/v1/app/*). É o único ator
// que lê conteúdo clínico em claro (via SealedText.Reveal() nos mappers). Não
// importa os pacotes dos outros atores (portal/admin) — segregação por ator (D2).
package app

import (
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// PatientDTO é a projeção pública de um paciente para o terapeuta.
type PatientDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	CPF            string `json:"cpf,omitempty"`
	OriginID       string `json:"origin_id,omitempty"`
	ApprovalStatus string `json:"approval_status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// AnamnesisDTO carrega o clínico em CLARO — só no ator app (D6.3).
type AnamnesisDTO struct {
	PatientID string `json:"patient_id"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

// NoteDTO carrega o clínico em CLARO — só no ator app (D6.3).
type NoteDTO struct {
	ID        string `json:"id"`
	PatientID string `json:"patient_id"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// PatientChartDTO é a resposta de GET /v1/app/patients/{id}: paciente +
// prontuário legível (anamnese opcional + notas em claro).
type PatientChartDTO struct {
	Patient   PatientDTO    `json:"patient"`
	Anamnesis *AnamnesisDTO `json:"anamnesis,omitempty"`
	Notes     []NoteDTO     `json:"notes"`
}

// SessionDTO é a projeção de uma sessão para o terapeuta.
type SessionDTO struct {
	ID         string `json:"id"`
	PatientID  string `json:"patient_id"`
	LocationID string `json:"location_id,omitempty"`
	Modality   string `json:"modality"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	Status     string `json:"status"`
	MeetURL    string `json:"meet_url,omitempty"`
}

// DebtDTO é a projeção de um débito. Dinheiro em centavos (int64).
type DebtDTO struct {
	ID            string `json:"id"`
	PatientID     string `json:"patient_id"`
	SessionID     string `json:"session_id,omitempty"`
	BillingPeriod string `json:"billing_period,omitempty"`
	AmountCents   int64  `json:"amount_cents"`
	DueDate       string `json:"due_date"`
	Status        string `json:"status"`
}

// PaymentDTO é a projeção de um pagamento. Dinheiro em centavos (int64).
type PaymentDTO struct {
	ID          string `json:"id"`
	DebtID      string `json:"debt_id"`
	AmountCents int64  `json:"amount_cents"`
	PaidAt      string `json:"paid_at"`
	Method      string `json:"method"`
}

// InvoiceDTO é a projeção do PDF de cobrança gerado (metadado no GED). O
// conteúdo (bytes) é servido pela rota de download, não neste DTO.
type InvoiceDTO struct {
	FileID    string `json:"file_id"`
	DebtID    string `json:"debt_id"`
	PatientID string `json:"patient_id"`
	MIME      string `json:"mime"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

// AISuggestionDTO é a resposta do disparo da IA: o resumo (em CLARO, só no ator
// app via Reveal) + as tags sugeridas para revisão. Status "sugerido".
type AISuggestionDTO struct {
	SummaryID     string   `json:"summary_id"`
	SessionID     string   `json:"session_id"`
	Summary       string   `json:"summary"`
	Status        string   `json:"status"`
	SuggestedTags []string `json:"suggested_tags"`
}

// AISummaryDTO é a projeção de um resumo de IA consolidado (resumo em CLARO).
type AISummaryDTO struct {
	SummaryID string `json:"summary_id"`
	SessionID string `json:"session_id"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
}

// TagSearchDTO é a resposta da busca por tag (sessões do paciente no ano).
type TagSearchDTO struct {
	Tag        string   `json:"tag"`
	Ano        int      `json:"ano"`
	SessionIDs []string `json:"session_ids"`
}

// PayResultDTO é a resposta de POST /v1/app/debts/{id}/pay.
type PayResultDTO struct {
	Debt    DebtDTO    `json:"debt"`
	Payment PaymentDTO `json:"payment"`
}

// FinancialSummaryDTO é a projeção do resumo financeiro. Centavos (int64).
type FinancialSummaryDTO struct {
	GeneratedCents int64 `json:"generated_cents"`
	OpenCents      int64 `json:"open_cents"`
	ReceivedCents  int64 `json:"received_cents"`
	OverdueCents   int64 `json:"overdue_cents"`
}

// ROIRowDTO é uma linha do relatório de ROI por canal. Centavos (int64).
type ROIRowDTO struct {
	OriginID     string `json:"origin_id"`
	OriginName   string `json:"origin_name"`
	RevenueCents int64  `json:"revenue_cents"`
	CostCents    int64  `json:"cost_cents"`
	ROICents     int64  `json:"roi_cents"`
}

// --- mappers (o Reveal() só aparece aqui, no ator app) ---

func toPatientDTO(p *patient.Patient) PatientDTO {
	return PatientDTO{
		ID:             p.ID,
		Name:           p.Name,
		Phone:          p.Phone,
		Email:          p.Email,
		CPF:            p.CPF,
		OriginID:       p.OriginID,
		ApprovalStatus: p.ApprovalStatus,
		CreatedAt:      httpx.FmtTime(p.CreatedAt),
		UpdatedAt:      httpx.FmtTime(p.UpdatedAt),
	}
}

func toPatientDTOs(ps []*patient.Patient) []PatientDTO {
	out := make([]PatientDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, toPatientDTO(p))
	}
	return out
}

func toAnamnesisDTO(a *clinical.Anamnesis) AnamnesisDTO {
	return AnamnesisDTO{
		PatientID: a.PatientID,
		Content:   a.Content.Reveal(),
		UpdatedAt: httpx.FmtTime(a.UpdatedAt),
	}
}

func toNoteDTO(n *clinical.Note) NoteDTO {
	return NoteDTO{
		ID:        n.ID,
		PatientID: n.PatientID,
		SessionID: n.SessionID,
		Content:   n.Content.Reveal(),
		CreatedAt: httpx.FmtTime(n.CreatedAt),
	}
}

func toNoteDTOs(ns []*clinical.Note) []NoteDTO {
	out := make([]NoteDTO, 0, len(ns))
	for _, n := range ns {
		out = append(out, toNoteDTO(n))
	}
	return out
}

func toSessionDTO(s *scheduling.Session) SessionDTO {
	return SessionDTO{
		ID:         s.ID,
		PatientID:  s.PatientID,
		LocationID: s.LocationID,
		Modality:   s.Modality,
		StartsAt:   httpx.FmtTime(s.StartsAt),
		EndsAt:     httpx.FmtTime(s.EndsAt),
		Status:     s.Status,
		MeetURL:    s.MeetURL,
	}
}

func toDebtDTO(d *billing.Debt) DebtDTO {
	return DebtDTO{
		ID:            d.ID,
		PatientID:     d.PatientID,
		SessionID:     d.SessionID,
		BillingPeriod: d.BillingPeriod,
		AmountCents:   d.Amount,
		DueDate:       httpx.FmtTime(d.DueDate),
		Status:        d.Status,
	}
}

func toInvoiceDTO(f *domain.GEDFile) InvoiceDTO {
	return InvoiceDTO{
		FileID:    f.ID,
		DebtID:    f.DebtID,
		PatientID: f.PatientID,
		MIME:      f.MIME,
		SizeBytes: f.Size,
		SHA256:    f.SHA256,
	}
}

func toDebtDTOs(ds []*billing.Debt) []DebtDTO {
	out := make([]DebtDTO, 0, len(ds))
	for _, d := range ds {
		out = append(out, toDebtDTO(d))
	}
	return out
}

func toPaymentDTO(p *billing.Payment) PaymentDTO {
	return PaymentDTO{
		ID:          p.ID,
		DebtID:      p.DebtID,
		AmountCents: p.Amount,
		PaidAt:      httpx.FmtTime(p.PaidAt),
		Method:      p.Method,
	}
}

func toFinancialSummaryDTO(f *costs.FinancialSummary) FinancialSummaryDTO {
	return FinancialSummaryDTO{
		GeneratedCents: f.Generated,
		OpenCents:      f.Open,
		ReceivedCents:  f.Received,
		OverdueCents:   f.Overdue,
	}
}

func toROIRowDTOs(rows []costs.ROIRow) []ROIRowDTO {
	out := make([]ROIRowDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, ROIRowDTO{
			OriginID:     r.OriginID,
			OriginName:   r.OriginName,
			RevenueCents: r.Revenue,
			CostCents:    r.Cost,
			ROICents:     r.ROI,
		})
	}
	return out
}
