package app

import (
	"context"
	"sort"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// DayFeedService compõe o feed do dia do terapeuta (R8). Transversal a todos os
// pacientes num intervalo, reaproveitando o motor de agregação e a janela do
// domínio clinical. O serviço só coordena o I/O das fontes.
// Portado de c3ab585:internal/service/dayfeed.go.
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

// Build compõe o feed do dia para a janela pedida.
func (s *DayFeedService) Build(ctx context.Context, window string) (*clinical.DayFeed, error) {
	from, to, window := clinical.WindowRange(window, s.clock.Now())
	// Nome do paciente para rotular os cards.
	patients, err := s.patients.List(ctx)
	if err != nil {
		return nil, err
	}
	name := map[string]string{}
	for _, p := range patients {
		name[p.ID] = p.Name
	}
	feed := &clinical.DayFeed{Window: window, Resumo: clinical.DayFeedResumo{Counts: map[string]int{}}}
	// Sessões por starts_at no intervalo.
	sessions, err := s.sessions.ListByRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: clinical.HistorySessao, Data: sess.StartsAt,
			PatientID: sess.PatientID, PatientName: name[sess.PatientID],
			Titulo: "Sessão", Resumo: sess.Modality, Status: sess.Status,
			Ref: clinical.HistoryRef{Kind: "session", ID: sess.ID},
		})
	}
	// Resumo: agregação das sessões do período (reusa o motor) + a receber.
	agg := clinical.AggregateSessions(sessions, nil)
	feed.Resumo.Counts = agg.Counts
	feed.Resumo.Total = agg.Total
	// Débitos criados no período.
	debts, err := s.debts.ListByCreatedRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, d := range debts {
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: clinical.HistoryDebito, Data: d.CreatedAt,
			PatientID: d.PatientID, PatientName: name[d.PatientID],
			Titulo: "Débito gerado", Resumo: clinical.BRL(d.Amount), Valor: d.Amount, Status: d.Status,
			Ref: clinical.HistoryRef{Kind: "debt", ID: d.ID},
		})
		if d.Status == billing.DebtAberto || d.Status == billing.DebtParcial {
			feed.Resumo.AReceber += d.Amount
		}
	}
	// Vencimentos próximos (débitos abertos/parciais vencendo no período).
	due, err := s.debts.ListDueBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, d := range due {
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: clinical.HistoryDebito, Data: d.DueDate,
			PatientID: d.PatientID, PatientName: name[d.PatientID],
			Titulo: "Vencimento de débito", Resumo: clinical.BRL(d.Amount), Valor: d.Amount, Status: d.Status,
			Ref: clinical.HistoryRef{Kind: "debt", ID: d.ID},
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
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: clinical.HistoryPagamento, Data: p.PaidAt,
			PatientID: pid, PatientName: name[pid],
			Titulo: "Pagamento recebido", Resumo: clinical.BRL(p.Amount), Valor: p.Amount,
			Ref: clinical.HistoryRef{Kind: "debt", ID: p.DebtID},
		})
		feed.Resumo.Recebido += p.Amount
	}
	// Arquivos anexados no período.
	files, err := s.ged.ListByCreatedRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: clinical.HistoryArquivo, Data: f.CreatedAt,
			PatientID: f.PatientID, PatientName: name[f.PatientID],
			Titulo: "Arquivo anexado", Resumo: f.MIME,
			Ref: clinical.HistoryRef{Kind: "ged_file", ID: f.ID},
		})
	}
	// Pedidos de agendamento pendentes (futuro/por fazer).
	pending, err := s.appointments.ListPending(ctx)
	if err != nil {
		return nil, err
	}
	for _, req := range pending {
		feed.Eventos = append(feed.Eventos, clinical.DayFeedEvent{
			Tipo: "pedido", Data: req.SlotStart,
			PatientID: req.PatientID, PatientName: name[req.PatientID],
			Titulo: "Pedido de agendamento", Resumo: "Aguardando confirmação", Status: req.Status,
			Ref: clinical.HistoryRef{Kind: "appointment_request", ID: req.ID},
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
