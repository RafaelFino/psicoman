package service

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// --- fakes dedicados ao feed do dia -----------------------------------------

type dfDebtRepo struct {
	created []*domain.Debt
	due     []*domain.Debt
}

func (f *dfDebtRepo) InsertIfAbsent(context.Context, *domain.Debt) (bool, error) { return true, nil }
func (f *dfDebtRepo) Get(context.Context, string) (*domain.Debt, error)          { return nil, ErrNotFound }
func (f *dfDebtRepo) GetByIdempotencyKey(context.Context, string) (*domain.Debt, error) {
	return nil, ErrNotFound
}
func (f *dfDebtRepo) List(context.Context) ([]*domain.Debt, error)                  { return nil, nil }
func (f *dfDebtRepo) ListByPatient(context.Context, string) ([]*domain.Debt, error) { return nil, nil }
func (f *dfDebtRepo) ListBySessionIDs(context.Context, []string) ([]*domain.Debt, error) {
	return nil, nil
}
func (f *dfDebtRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return f.created, nil
}
func (f *dfDebtRepo) ListDueBetween(context.Context, time.Time, time.Time) ([]*domain.Debt, error) {
	return f.due, nil
}
func (f *dfDebtRepo) UpdateStatus(context.Context, string, string) error { return nil }
func (f *dfDebtRepo) SetPDF(context.Context, string, string) error       { return nil }

type dfPaymentRepo struct{ pays []*domain.Payment }

func (f *dfPaymentRepo) Insert(context.Context, *domain.Payment) error { return nil }
func (f *dfPaymentRepo) ListByDebt(context.Context, string) ([]*domain.Payment, error) {
	return nil, nil
}
func (f *dfPaymentRepo) SumByDebt(context.Context, string) (int64, error) { return 0, nil }
func (f *dfPaymentRepo) ListByRange(context.Context, time.Time, time.Time) ([]*domain.Payment, error) {
	return f.pays, nil
}

type dfGEDRepo struct{ files []*domain.GEDFile }

func (f *dfGEDRepo) Insert(context.Context, *domain.GEDFile) error        { return nil }
func (f *dfGEDRepo) Get(context.Context, string) (*domain.GEDFile, error) { return nil, ErrNotFound }
func (f *dfGEDRepo) FindByHash(context.Context, string, string) (*domain.GEDFile, error) {
	return nil, ErrNotFound
}
func (f *dfGEDRepo) ListByPatient(context.Context, string) ([]*domain.GEDFile, error) {
	return nil, nil
}
func (f *dfGEDRepo) ListByCreatedRange(context.Context, time.Time, time.Time) ([]*domain.GEDFile, error) {
	return f.files, nil
}
func (f *dfGEDRepo) Delete(context.Context, string) error { return nil }

type dfApptRepo struct{ pending []*domain.AppointmentRequest }

func (f *dfApptRepo) Create(context.Context, *domain.AppointmentRequest) error { return nil }
func (f *dfApptRepo) Get(context.Context, string) (*domain.AppointmentRequest, error) {
	return nil, ErrNotFound
}
func (f *dfApptRepo) ListPending(context.Context) ([]*domain.AppointmentRequest, error) {
	return f.pending, nil
}
func (f *dfApptRepo) ListByPatient(context.Context, string) ([]*domain.AppointmentRequest, error) {
	return nil, nil
}
func (f *dfApptRepo) UpdateStatus(context.Context, string, string) error { return nil }

func TestDayFeedBuild(t *testing.T) {
	loc := clock.Location()
	// Relógio fixo "hoje" às 12:00.
	fixed := clock.Fixed{T: time.Date(2026, 3, 10, 12, 0, 0, 0, loc)}

	patients := newFakePatientRepo()
	patients.put(&domain.Patient{ID: "pat-1", Name: "Ana", Email: "ana@x.com"})

	sessions := newFakeSessionRepo()
	// Sessão de hoje de manhã (realizada) e outra de hoje à tarde (agendada).
	sessions.put(&domain.Session{
		ID: "s1", PatientID: "pat-1", Modality: domain.ModalityOnline,
		StartsAt: time.Date(2026, 3, 10, 9, 0, 0, 0, loc), EndsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, loc),
		Status: domain.SessionRealizada,
	})
	sessions.put(&domain.Session{
		ID: "s2", PatientID: "pat-1", Modality: domain.ModalityPresencial,
		StartsAt: time.Date(2026, 3, 10, 16, 0, 0, 0, loc), EndsAt: time.Date(2026, 3, 10, 17, 0, 0, 0, loc),
		Status: domain.SessionAgendada,
	})
	// Sessão de ontem (fora da janela "hoje").
	sessions.put(&domain.Session{
		ID: "s0", PatientID: "pat-1", Modality: domain.ModalityOnline,
		StartsAt: time.Date(2026, 3, 9, 9, 0, 0, 0, loc), EndsAt: time.Date(2026, 3, 9, 10, 0, 0, 0, loc),
		Status: domain.SessionRealizada,
	})

	debts := &dfDebtRepo{
		created: []*domain.Debt{{ID: "d1", PatientID: "pat-1", Amount: 15000, Status: domain.DebtAberto, CreatedAt: time.Date(2026, 3, 10, 10, 5, 0, 0, loc)}},
	}
	payments := &dfPaymentRepo{
		pays: []*domain.Payment{{ID: "p1", DebtID: "d1", Amount: 15000, Method: "pix", PaidAt: time.Date(2026, 3, 10, 11, 0, 0, 0, loc)}},
	}
	ged := &dfGEDRepo{
		files: []*domain.GEDFile{{ID: "f1", PatientID: "pat-1", MIME: "application/pdf", CreatedAt: time.Date(2026, 3, 10, 11, 30, 0, 0, loc)}},
	}
	appts := &dfApptRepo{
		pending: []*domain.AppointmentRequest{{ID: "r1", PatientID: "pat-1", Status: domain.ApptPendente, SlotStart: time.Date(2026, 3, 12, 10, 0, 0, 0, loc)}},
	}

	svc := NewDayFeedService(sessions, debts, payments, ged, appts, patients, NewAuditService(fakeAuditRepo{}))
	svc.clock = fixed

	feed, err := svc.Build(context.Background(), DayWindowHoje)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if feed.Window != DayWindowHoje {
		t.Errorf("window = %q", feed.Window)
	}

	// Conta tipos nos eventos.
	tipos := map[string]int{}
	for _, e := range feed.Eventos {
		tipos[e.Tipo]++
		if e.PatientName != "Ana" && e.Tipo != "pedido" {
			// pedido também é da Ana, mas o nome deve bater sempre que houver paciente
		}
	}
	// 2 sessões de hoje (a de ontem fica fora), 1 débito, 1 pagamento, 1 arquivo, 1 pedido.
	if tipos[HistorySessao] != 2 {
		t.Errorf("sessões = %d, quer 2 (ontem fora da janela)", tipos[HistorySessao])
	}
	if tipos[HistoryDebito] != 1 {
		t.Errorf("débitos = %d, quer 1", tipos[HistoryDebito])
	}
	if tipos[HistoryPagamento] != 1 {
		t.Errorf("pagamentos = %d, quer 1", tipos[HistoryPagamento])
	}
	if tipos[HistoryArquivo] != 1 {
		t.Errorf("arquivos = %d, quer 1", tipos[HistoryArquivo])
	}
	if tipos["pedido"] != 1 {
		t.Errorf("pedidos = %d, quer 1", tipos["pedido"])
	}

	// Resumo: recebido = 15000; a_receber = 15000 (débito aberto criado hoje);
	// total de sessões no período = 2; realizadas = 1.
	if feed.Resumo.Recebido != 15000 {
		t.Errorf("recebido = %d, quer 15000", feed.Resumo.Recebido)
	}
	if feed.Resumo.AReceber != 15000 {
		t.Errorf("a_receber = %d, quer 15000", feed.Resumo.AReceber)
	}
	if feed.Resumo.Total != 2 {
		t.Errorf("total sessões = %d, quer 2", feed.Resumo.Total)
	}
	if feed.Resumo.Counts[domain.CategoryRealizada] != 1 {
		t.Errorf("realizadas no resumo = %d, quer 1", feed.Resumo.Counts[domain.CategoryRealizada])
	}

	// Ordenação desc por data.
	for i := 1; i < len(feed.Eventos); i++ {
		if feed.Eventos[i-1].Data.Before(feed.Eventos[i].Data) {
			t.Errorf("ordem não-desc em %d/%d", i-1, i)
		}
	}

	// Nome do paciente propagado nos cards.
	for _, e := range feed.Eventos {
		if e.PatientID == "pat-1" && e.PatientName != "Ana" {
			t.Errorf("evento %q sem nome do paciente", e.Tipo)
		}
	}
}

func TestDayFeedWindows(t *testing.T) {
	loc := clock.Location()
	fixed := clock.Fixed{T: time.Date(2026, 3, 10, 12, 0, 0, 0, loc)}
	patients := newFakePatientRepo()
	sessions := newFakeSessionRepo()
	svc := NewDayFeedService(sessions, &dfDebtRepo{}, &dfPaymentRepo{}, &dfGEDRepo{}, &dfApptRepo{}, patients, NewAuditService(fakeAuditRepo{}))
	svc.clock = fixed

	// proximos7: [hoje 00:00, hoje+8d).
	from, to := svc.windowRange(DayWindowProximos7)
	if from.Hour() != 0 || to.Sub(from) != 8*24*time.Hour {
		t.Errorf("janela proximos7 inesperada: %v → %v", from, to)
	}
	// recentes: [hoje-7d, hoje+1d).
	rf, rt := svc.windowRange(DayWindowRecentes)
	if rt.Sub(rf) != 8*24*time.Hour {
		t.Errorf("janela recentes inesperada: %v → %v", rf, rt)
	}
}
