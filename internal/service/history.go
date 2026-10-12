package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// Tipos de evento da timeline do paciente (patient-timeline D4.1).
const (
	HistoryCadastro   = "cadastro"
	HistoryAprovacao  = "aprovacao"
	HistorySessao     = "sessao"
	HistoryRemarcacao = "remarcacao"
	HistoryRegistro   = "registro"   // nota vinculada a uma sessão
	HistoryNotaLivre  = "nota_livre" // nota sem sessão
	HistoryAnamnese   = "anamnese"
	HistoryDebito     = "debito"
	HistoryPagamento  = "pagamento"
	HistoryArquivo    = "arquivo"
)

// Categorias de filtro da timeline (patient-timeline D4.2).
const (
	HistoryCatClinico    = "clinico"
	HistoryCatFinanceiro = "financeiro"
	HistoryCatAgenda     = "agenda"
	HistoryCatArquivos   = "arquivos"
	HistoryCatGeral      = "geral"
)

// HistoryRef aponta para a entidade de origem de um evento (para a ação da UI).
type HistoryRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// HistoryItem é um evento normalizado da timeline do paciente. Nunca carrega
// conteúdo clínico cru em metadados de auditoria (R4.5); o resumo é texto
// legível exibido na UI.
type HistoryItem struct {
	Tipo      string     `json:"tipo"`
	Categoria string     `json:"categoria"`
	Data      time.Time  `json:"data"`
	Titulo    string     `json:"titulo"`
	Resumo    string     `json:"resumo"`
	Ref       HistoryRef `json:"ref"`
}

// HistoryService compõe, no servidor, a história completa de um paciente a
// partir das fontes já existentes (sessões, notas, anamnese, débitos,
// pagamentos, arquivos, aprovação) e das remarcações registradas no audit log.
// A composição é derivada: não há tabela de "eventos" (patient-timeline D4.1).
type HistoryService struct {
	patients PatientRepository
	sessions SessionRepository
	notes    ProntuarioRepository
	debts    DebtRepository
	payments PaymentRepository
	ged      GEDMetaRepository
	audit    *AuditService
}

// NewHistoryService cria o serviço de composição do histórico.
func NewHistoryService(
	patients PatientRepository,
	sessions SessionRepository,
	notes ProntuarioRepository,
	debts DebtRepository,
	payments PaymentRepository,
	ged GEDMetaRepository,
	audit *AuditService,
) *HistoryService {
	return &HistoryService{
		patients: patients, sessions: sessions, notes: notes,
		debts: debts, payments: payments, ged: ged, audit: audit,
	}
}

// PatientHistory devolve os eventos do paciente, ordenados do mais recente ao
// mais antigo. A leitura agrega conteúdo clínico (notas/anamnese), portanto
// gera um registro de auditoria AuditActionRead (metadata sem conteúdo clínico).
func (s *HistoryService) PatientHistory(ctx context.Context, patientID string) ([]HistoryItem, error) {
	patient, err := s.patients.Get(ctx, patientID)
	if err != nil {
		return nil, err
	}

	var items []HistoryItem

	// Cadastro / primeiro contato.
	items = append(items, HistoryItem{
		Tipo: HistoryCadastro, Categoria: HistoryCatGeral, Data: patient.CreatedAt,
		Titulo: "Cadastro do paciente", Resumo: "Início da relação no consultório.",
		Ref: HistoryRef{Kind: "patient", ID: patient.ID},
	})
	// Aprovação (quando aprovado/rejeitado — estado atual do paciente).
	if patient.ApprovalStatus == domain.PatientApproved || patient.ApprovalStatus == domain.PatientRejected {
		titulo := "Acesso ao portal aprovado"
		if patient.ApprovalStatus == domain.PatientRejected {
			titulo = "Acesso ao portal rejeitado"
		}
		items = append(items, HistoryItem{
			Tipo: HistoryAprovacao, Categoria: HistoryCatGeral, Data: patient.UpdatedAt,
			Titulo: titulo, Resumo: "", Ref: HistoryRef{Kind: "patient", ID: patient.ID},
		})
	}

	// Sessões + remarcações (do audit por sessão).
	sessions, err := s.sessions.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		items = append(items, sessionItem(sess))
		// Remarcações registradas no audit log desta sessão.
		if s.audit != nil {
			entries, aerr := s.audit.ListByEntity(ctx, "session", sess.ID)
			if aerr == nil {
				for _, e := range entries {
					if e.Action != domain.AuditActionSessionReschedule {
						continue
					}
					items = append(items, rescheduleItem(sess, e))
				}
			}
		}
	}

	// Notas (registro de sessão / nota livre) e anamnese.
	notes, err := s.notes.ListNotes(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		items = append(items, noteItem(n))
	}
	if anam, aerr := s.notes.GetAnamnesis(ctx, patientID); aerr == nil && anam != nil && anam.Content != "" {
		data := anam.UpdatedAt
		if data.IsZero() {
			data = anam.CreatedAt
		}
		items = append(items, HistoryItem{
			Tipo: HistoryAnamnese, Categoria: HistoryCatClinico, Data: data,
			Titulo: "Anamnese", Resumo: "Ficha inicial da história clínica.",
			Ref: HistoryRef{Kind: "anamnesis", ID: patient.ID},
		})
	}

	// Débitos + pagamentos.
	debts, err := s.debts.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, d := range debts {
		items = append(items, debtItem(d))
		if s.payments != nil {
			pays, perr := s.payments.ListByDebt(ctx, d.ID)
			if perr == nil {
				for _, p := range pays {
					items = append(items, paymentItem(d, p))
				}
			}
		}
	}

	// Arquivos (GED).
	files, err := s.ged.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		items = append(items, fileItem(f))
	}

	// Ordenação desc por data (mais recente no topo).
	sort.SliceStable(items, func(i, j int) bool { return items[i].Data.After(items[j].Data) })

	// Auditoria de leitura clínica (R4.5): sem conteúdo em metadata.
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionRead, "patient_history", patientID,
			map[string]any{"patient_id": patientID, "itens": len(items)})
	}
	return items, nil
}

func sessionItem(s *domain.Session) HistoryItem {
	resumo := fmt.Sprintf("%s · %s", s.Modality, sessionStatusLabel(s.Status))
	if s.Status == domain.SessionCancelada && s.CanceledBy != "" {
		resumo += " · por " + s.CanceledBy
		if s.CancelReason != "" {
			resumo += " (" + s.CancelReason + ")"
		}
	}
	return HistoryItem{
		Tipo: HistorySessao, Categoria: HistoryCatAgenda, Data: s.StartsAt,
		Titulo: "Sessão", Resumo: resumo, Ref: HistoryRef{Kind: "session", ID: s.ID},
	}
}

func rescheduleItem(s *domain.Session, e *domain.AuditLog) HistoryItem {
	resumo := "Horário da sessão alterado."
	if e.Metadata != nil {
		from, _ := e.Metadata["from"].(string)
		to, _ := e.Metadata["to"].(string)
		if from != "" && to != "" {
			resumo = fmt.Sprintf("De %s para %s.", from, to)
		}
	}
	return HistoryItem{
		Tipo: HistoryRemarcacao, Categoria: HistoryCatAgenda, Data: e.CreatedAt,
		Titulo: "Remarcação", Resumo: resumo, Ref: HistoryRef{Kind: "session", ID: s.ID},
	}
}

func noteItem(n *domain.Note) HistoryItem {
	if n.IsFree() {
		return HistoryItem{
			Tipo: HistoryNotaLivre, Categoria: HistoryCatClinico, Data: n.CreatedAt,
			Titulo: "Nota", Resumo: "Anotação livre do terapeuta.",
			Ref: HistoryRef{Kind: "note", ID: n.ID},
		}
	}
	return HistoryItem{
		Tipo: HistoryRegistro, Categoria: HistoryCatClinico, Data: n.CreatedAt,
		Titulo: "Registro da sessão", Resumo: "Relatório clínico vinculado à sessão.",
		Ref: HistoryRef{Kind: "session", ID: n.SessionID},
	}
}

func debtItem(d *domain.Debt) HistoryItem {
	origem := "sessão"
	if d.BillingPeriod != "" {
		origem = "ciclo " + d.BillingPeriod
	}
	return HistoryItem{
		Tipo: HistoryDebito, Categoria: HistoryCatFinanceiro, Data: d.CreatedAt,
		Titulo: "Débito gerado",
		Resumo: fmt.Sprintf("%s · %s", brl(d.Amount), origem),
		Ref:    HistoryRef{Kind: "debt", ID: d.ID},
	}
}

func paymentItem(d *domain.Debt, p *domain.Payment) HistoryItem {
	resumo := brl(p.Amount)
	if p.Method != "" {
		resumo += " · " + p.Method
	}
	return HistoryItem{
		Tipo: HistoryPagamento, Categoria: HistoryCatFinanceiro, Data: p.PaidAt,
		Titulo: "Pagamento registrado", Resumo: resumo,
		Ref: HistoryRef{Kind: "debt", ID: d.ID},
	}
}

func fileItem(f *domain.GEDFile) HistoryItem {
	return HistoryItem{
		Tipo: HistoryArquivo, Categoria: HistoryCatArquivos, Data: f.CreatedAt,
		Titulo: "Arquivo anexado", Resumo: f.MIME,
		Ref: HistoryRef{Kind: "ged_file", ID: f.ID},
	}
}

// sessionStatusLabel devolve o rótulo PT-BR do status (espelha Fmt.sessionStatus).
func sessionStatusLabel(status string) string {
	switch status {
	case domain.SessionSolicitada:
		return "Solicitada"
	case domain.SessionAgendada:
		return "Agendada"
	case domain.SessionConfirmada:
		return "Confirmada"
	case domain.SessionRealizada:
		return "Realizada"
	case domain.SessionCancelada:
		return "Cancelada"
	case domain.SessionFalta:
		return "Falta"
	default:
		return status
	}
}

// brl formata centavos em reais (R$ 1.234,56) para os resumos textuais.
func brl(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	reais := cents / 100
	cent := cents % 100
	s := fmt.Sprintf("R$ %d,%02d", reais, cent)
	if neg {
		s = "-" + s
	}
	return s
}
