package service

import (
	"context"
	"sort"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// Janelas temporais do feed do dia (patient-timeline R8.1).
const (
	DayWindowHoje      = "hoje"
	DayWindowProximos7 = "proximos7"
	DayWindowRecentes  = "recentes"
)

// DayFeedEvent é um evento normalizado do feed do dia, transversal a todos os
// pacientes. Reaproveita os tipos da timeline por-paciente (history.go), mas
// carrega o paciente associado (R8.3). Nunca traz conteúdo clínico.
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

// DayFeedService compõe o feed do dia do terapeuta (R8). Transversal a todos os
// pacientes num intervalo, reaproveitando o motor de agregação (F1).
type DayFeedService struct {
	sessions     SessionRepository
	debts        DebtRepository
	payments     PaymentRepository
	ged          GEDMetaRepository
	appointments AppointmentRepository
	patients     PatientRepository
	audit        *AuditService
	clock        clock.Clock
}

// NewDayFeedService cria o serviço do feed do dia.
func NewDayFeedService(
	sessions SessionRepository, debts DebtRepository, payments PaymentRepository,
	ged GEDMetaRepository, appointments AppointmentRepository, patients PatientRepository,
	audit *AuditService,
) *DayFeedService {
	return &DayFeedService{
		sessions: sessions, debts: debts, payments: payments, ged: ged,
		appointments: appointments, patients: patients, audit: audit, clock: clock.System{},
	}
}

// windowRange devolve [from, to) para a janela pedida, no fuso do projeto.
func (s *DayFeedService) windowRange(window string) (from, to time.Time) {
	now := s.clock.Now()
	loc := clock.Location()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch window {
	case DayWindowProximos7:
		return startOfDay, startOfDay.AddDate(0, 0, 8) // hoje + 7 dias
	case DayWindowRecentes:
		return startOfDay.AddDate(0, 0, -7), startOfDay.AddDate(0, 0, 1)
	default: // hoje
		return startOfDay, startOfDay.AddDate(0, 0, 1)
	}
}

// Build compõe o feed do dia para a janela pedida.
func (s *DayFeedService) Build(ctx context.Context, window string) (*DayFeed, error) {
	if window != DayWindowProximos7 && window != DayWindowRecentes {
		window = DayWindowHoje
	}
	from, to := s.windowRange(window)

	// Nome do paciente para rotular os cards.
	patients, err := s.patients.List(ctx)
	if err != nil {
		return nil, err
	}
	name := map[string]string{}
	for _, p := range patients {
		name[p.ID] = p.Name
	}

	feed := &DayFeed{Window: window, Resumo: DayFeedResumo{Counts: map[string]int{}}}

	// Sessões por starts_at no intervalo.
	sessions, err := s.sessions.ListByRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: HistorySessao, Data: sess.StartsAt,
			PatientID: sess.PatientID, PatientName: name[sess.PatientID],
			Titulo: "Sessão", Resumo: sess.Modality, Status: sess.Status,
			Ref: HistoryRef{Kind: "session", ID: sess.ID},
		})
	}
	// Resumo: agregação das sessões do período (reusa F1) + a receber.
	agg := aggregateSessions(sessions, nil)
	feed.Resumo.Counts = agg.Counts
	feed.Resumo.Total = agg.Total

	// Débitos criados no período.
	debts, err := s.debts.ListByCreatedRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, d := range debts {
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: HistoryDebito, Data: d.CreatedAt,
			PatientID: d.PatientID, PatientName: name[d.PatientID],
			Titulo: "Débito gerado", Resumo: brl(d.Amount), Valor: d.Amount, Status: d.Status,
			Ref: HistoryRef{Kind: "debt", ID: d.ID},
		})
		if d.Status == domain.DebtAberto || d.Status == domain.DebtParcial {
			feed.Resumo.AReceber += d.Amount
		}
	}

	// Vencimentos próximos (débitos abertos/parciais vencendo no período).
	due, err := s.debts.ListDueBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, d := range due {
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: HistoryDebito, Data: d.DueDate,
			PatientID: d.PatientID, PatientName: name[d.PatientID],
			Titulo: "Vencimento de débito", Resumo: brl(d.Amount), Valor: d.Amount, Status: d.Status,
			Ref: HistoryRef{Kind: "debt", ID: d.ID},
		})
	}

	// Pagamentos recebidos no período.
	payments, err := s.payments.ListByRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	// Mapa débito->paciente para rotular os pagamentos.
	debtPatient := map[string]string{}
	for _, d := range debts {
		debtPatient[d.ID] = d.PatientID
	}
	for _, p := range payments {
		pid := debtPatient[p.DebtID]
		if pid == "" {
			if d, derr := s.debts.Get(ctx, p.DebtID); derr == nil {
				pid = d.PatientID
			}
		}
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: HistoryPagamento, Data: p.PaidAt,
			PatientID: pid, PatientName: name[pid],
			Titulo: "Pagamento recebido", Resumo: brl(p.Amount), Valor: p.Amount,
			Ref: HistoryRef{Kind: "debt", ID: p.DebtID},
		})
		feed.Resumo.Recebido += p.Amount
	}

	// Arquivos anexados no período.
	files, err := s.ged.ListByCreatedRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: HistoryArquivo, Data: f.CreatedAt,
			PatientID: f.PatientID, PatientName: name[f.PatientID],
			Titulo: "Arquivo anexado", Resumo: f.MIME,
			Ref: HistoryRef{Kind: "ged_file", ID: f.ID},
		})
	}

	// Pedidos de agendamento pendentes (futuro/por fazer).
	pending, err := s.appointments.ListPending(ctx)
	if err != nil {
		return nil, err
	}
	for _, req := range pending {
		feed.Eventos = append(feed.Eventos, DayFeedEvent{
			Tipo: "pedido", Data: req.SlotStart,
			PatientID: req.PatientID, PatientName: name[req.PatientID],
			Titulo: "Pedido de agendamento", Resumo: "Aguardando confirmação", Status: req.Status,
			Ref: HistoryRef{Kind: "appointment_request", ID: req.ID},
		})
	}

	// Ordenação desc por data (mais recente/próximo no topo).
	sort.SliceStable(feed.Eventos, func(i, j int) bool { return feed.Eventos[i].Data.After(feed.Eventos[j].Data) })

	// Auditoria de leitura (sem conteúdo clínico).
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionRead, "day_feed", window,
			map[string]any{"window": window, "eventos": len(feed.Eventos)})
	}
	return feed, nil
}
