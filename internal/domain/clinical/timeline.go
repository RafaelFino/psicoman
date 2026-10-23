package clinical

import (
	"fmt"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
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
// conteúdo clínico cru (R4.5); o resumo é texto legível de rótulos fixos — o
// SealedText das notas/anamnese NÃO é revelado aqui.
type HistoryItem struct {
	Tipo      string     `json:"tipo"`
	Categoria string     `json:"categoria"`
	Data      time.Time  `json:"data"`
	Titulo    string     `json:"titulo"`
	Resumo    string     `json:"resumo"`
	Ref       HistoryRef `json:"ref"`
}

// SessionItem monta o item de timeline de uma sessão.
func SessionItem(s *scheduling.Session) HistoryItem {
	resumo := fmt.Sprintf("%s · %s", s.Modality, SessionStatusLabel(s.Status))
	if s.Status == scheduling.SessionCancelada && s.CanceledBy != "" {
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

// RescheduleItem monta o item de remarcação a partir de um registro de audit.
func RescheduleItem(s *scheduling.Session, e *domain.AuditLog) HistoryItem {
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

// NoteItem monta o item de timeline de uma nota. Nunca revela o conteúdo
// clínico (SealedText) — usa só rótulos fixos (R4.3/R6.3).
func NoteItem(n *Note) HistoryItem {
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

// DebtItem monta o item de timeline de um débito gerado.
func DebtItem(d *billing.Debt) HistoryItem {
	origem := "sessão"
	if d.BillingPeriod != "" {
		origem = "ciclo " + d.BillingPeriod
	}
	return HistoryItem{
		Tipo: HistoryDebito, Categoria: HistoryCatFinanceiro, Data: d.CreatedAt,
		Titulo: "Débito gerado",
		Resumo: fmt.Sprintf("%s · %s", BRL(d.Amount), origem),
		Ref:    HistoryRef{Kind: "debt", ID: d.ID},
	}
}

// PaymentItem monta o item de timeline de um pagamento registrado.
func PaymentItem(d *billing.Debt, p *billing.Payment) HistoryItem {
	resumo := BRL(p.Amount)
	if p.Method != "" {
		resumo += " · " + p.Method
	}
	return HistoryItem{
		Tipo: HistoryPagamento, Categoria: HistoryCatFinanceiro, Data: p.PaidAt,
		Titulo: "Pagamento registrado", Resumo: resumo,
		Ref: HistoryRef{Kind: "debt", ID: d.ID},
	}
}

// FileItem monta o item de timeline de um arquivo anexado (GED).
func FileItem(f *domain.GEDFile) HistoryItem {
	return HistoryItem{
		Tipo: HistoryArquivo, Categoria: HistoryCatArquivos, Data: f.CreatedAt,
		Titulo: "Arquivo anexado", Resumo: f.MIME,
		Ref: HistoryRef{Kind: "ged_file", ID: f.ID},
	}
}

// SessionStatusLabel devolve o rótulo PT-BR do status da sessão.
func SessionStatusLabel(status string) string {
	switch status {
	case scheduling.SessionSolicitada:
		return "Solicitada"
	case scheduling.SessionAgendada:
		return "Agendada"
	case scheduling.SessionConfirmada:
		return "Confirmada"
	case scheduling.SessionRealizada:
		return "Realizada"
	case scheduling.SessionCancelada:
		return "Cancelada"
	case scheduling.SessionFalta:
		return "Falta"
	default:
		return status
	}
}

// BRL formata centavos em reais (R$ 1.234,56) para os resumos textuais.
func BRL(cents int64) string {
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

// --- Feed do dia (tipos e janela puros; R8) ---

// Janelas temporais do feed do dia (patient-timeline R8.1).
const (
	DayWindowHoje      = "hoje"
	DayWindowProximos7 = "proximos7"
	DayWindowRecentes  = "recentes"
)

// DayFeedEvent é um evento normalizado do feed do dia, transversal a todos os
// pacientes. Nunca traz conteúdo clínico.
type DayFeedEvent struct {
	Tipo        string     `json:"tipo"`
	Data        time.Time  `json:"data"`
	PatientID   string     `json:"patient_id"`
	PatientName string     `json:"patient_name"`
	Titulo      string     `json:"titulo"`
	Resumo      string     `json:"resumo"`
	Status      string     `json:"status,omitempty"`
	Valor       int64      `json:"valor,omitempty"` // centavos, quando financeiro
	Ref         HistoryRef `json:"ref"`
}

// DayFeedResumo é o bloco agregado do período (nasce colapsado na UI).
type DayFeedResumo struct {
	Counts   map[string]int `json:"counts"` // categoria de sessão -> nº
	Total    int            `json:"total"`
	Recebido int64          `json:"recebido"`  // soma dos pagamentos no período
	AReceber int64          `json:"a_receber"` // soma dos débitos abertos criados no período
}

// DayFeed é a resposta do endpoint: resumo + eventos ordenados.
type DayFeed struct {
	Window  string         `json:"window"`
	Resumo  DayFeedResumo  `json:"resumo"`
	Eventos []DayFeedEvent `json:"eventos"`
}

// WindowRange devolve [from, to) para a janela pedida, a partir de now, no fuso
// do projeto. Função pura (recebe o relógio por now). Portado de
// service/dayfeed.go:windowRange.
func WindowRange(window string, now time.Time) (from, to time.Time, normalized string) {
	if window != DayWindowProximos7 && window != DayWindowRecentes {
		window = DayWindowHoje
	}
	loc := now.Location()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch window {
	case DayWindowProximos7:
		return startOfDay, startOfDay.AddDate(0, 0, 8), window // hoje + 7 dias
	case DayWindowRecentes:
		return startOfDay.AddDate(0, 0, -7), startOfDay.AddDate(0, 0, 1), window
	default: // hoje
		return startOfDay, startOfDay.AddDate(0, 0, 1), window
	}
}
