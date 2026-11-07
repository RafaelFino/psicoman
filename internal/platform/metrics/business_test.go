package metrics

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
)

// --- famílias de negócio ---

func TestAppMetrics_Familias(t *testing.T) {
	o := newObs(false)
	m := o.App()

	m.Patient(app.PatientRegistered)
	m.Patient(app.PatientApproved)
	m.Session(app.SessionCreated)
	m.Session(app.SessionNoShow)
	m.Appointment(app.ApptConfirmed)
	m.DebtGenerated()
	m.PaymentRecorded()
	m.InvoicePDFGenerated()
	m.AuditRecorded("ler")
	m.AuditFailed("criar")
	m.CalendarDegraded()
	m.CipherOp(app.CipherSeal, app.ResultOK)
	m.CipherOp(app.CipherOpen, app.ResultError)
	m.AIRequest(app.ResultOK, 300*time.Millisecond)
	m.CalendarCall(app.CalendarFreeBusy, app.ResultConflict, 50*time.Millisecond)

	out := scrape(t, o)
	for _, want := range []string{
		`psicoman_pacientes_eventos_total{evento="cadastrado"} 1`,
		`psicoman_pacientes_eventos_total{evento="aprovado"} 1`,
		`psicoman_sessoes_eventos_total{evento="criada"} 1`,
		`psicoman_sessoes_eventos_total{evento="falta"} 1`,
		`psicoman_agendamentos_eventos_total{evento="confirmado"} 1`,
		`psicoman_debitos_gerados_total 1`,
		`psicoman_pagamentos_registrados_total 1`,
		`psicoman_pdfs_gerados_total 1`,
		`psicoman_audit_log_total{acao="ler"} 1`,
		`psicoman_audit_log_falhas_total{acao="criar"} 1`,
		`psicoman_calendar_degradado_total 1`,
		`psicoman_cifra_operacoes_total{op="seal",resultado="ok"} 1`,
		`psicoman_cifra_operacoes_total{op="open",resultado="erro"} 1`,
		`psicoman_gemini_chamadas_total{resultado="ok"} 1`,
		`psicoman_gemini_duracao_segundos_count{resultado="ok"} 1`,
		`psicoman_calendar_chamadas_total{op="freebusy",resultado="conflito"} 1`,
		`psicoman_calendar_duracao_segundos_count{op="freebusy"} 1`,
		`psicoman_gemini_ligada 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposição sem %q", want)
		}
	}
}

// TestAppMetrics_AcaoDeAuditoriaForaDoEnumViraOutro: um valor arbitrário (ex.:
// texto de usuário) NUNCA vira rótulo — cai em "outro".
func TestAppMetrics_AcaoDeAuditoriaForaDoEnumViraOutro(t *testing.T) {
	o := newObs(false)
	o.App().AuditRecorded("paciente-joao-silva@exemplo.com")
	out := scrape(t, o)
	if strings.Contains(out, "joao") {
		t.Fatal("texto livre vazou para um rótulo")
	}
	if !strings.Contains(out, `psicoman_audit_log_total{acao="outro"} 1`) {
		t.Error("ação desconhecida deveria virar acao=\"outro\"")
	}
}

func TestObs_GaugesDeIntegracao(t *testing.T) {
	ok := true
	o := New(Config{AIEnabled: true, GoogleOK: func() bool { return ok }})
	out := scrape(t, o)
	if !strings.Contains(out, "psicoman_gemini_ligada 1") || !strings.Contains(out, "psicoman_google_oauth_ok 1") {
		t.Errorf("gemini_ligada/google_oauth_ok deveriam ser 1:\n%s", out)
	}
	ok = false
	if !strings.Contains(scrape(t, o), "psicoman_google_oauth_ok 0") {
		t.Error("google_oauth_ok deveria acompanhar o estado (0 quando degradado)")
	}

	// Sem GoogleOK a família é omitida.
	if strings.Contains(scrape(t, newObs(false)), "psicoman_google_oauth_ok") {
		t.Error("sem fonte, google_oauth_ok deve ser omitida")
	}
}

// --- filas ---

type fakeQueues struct {
	mu          sync.Mutex
	patients    int
	appts       int
	patientsErr error
	apptsErr    error
	calls       int
}

func (f *fakeQueues) PendingPatients(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.patients, f.patientsErr
}
func (f *fakeQueues) PendingAppointments(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.appts, f.apptsErr
}

// stepClock é um clock.Clock avançável.
type stepClock struct{ t time.Time }

func (c *stepClock) Now() time.Time { return c.t }

func TestQueueCollector_LeEMostraGauges(t *testing.T) {
	q := &fakeQueues{patients: 3, appts: 5}
	o := New(Config{Queues: q, Clock: &stepClock{t: time.Unix(1_700_000_000, 0)}})
	out := scrape(t, o)
	if !strings.Contains(out, "psicoman_fila_pacientes_pendentes 3") ||
		!strings.Contains(out, "psicoman_fila_agendamentos_pendentes 5") {
		t.Errorf("gauges de fila ausentes ou errados:\n%s", out)
	}
}

func TestQueueCollector_CacheDe15sComRelogioFake(t *testing.T) {
	clk := &stepClock{t: time.Unix(1_700_000_000, 0)}
	q := &fakeQueues{patients: 1, appts: 1}
	o := New(Config{Queues: q, Clock: clk})

	scrape(t, o)
	scrape(t, o) // dentro do TTL: não relê
	if q.calls != 1 {
		t.Fatalf("dentro de 15 s a fonte deve ser lida 1 vez, foi %d", q.calls)
	}
	q.patients = 9
	clk.t = clk.t.Add(16 * time.Second)
	out := scrape(t, o) // TTL expirado: relê
	if q.calls != 2 {
		t.Errorf("após 15 s a fonte deve ser relida, calls=%d", q.calls)
	}
	if !strings.Contains(out, "psicoman_fila_pacientes_pendentes 9") {
		t.Error("o valor novo deveria aparecer após o cache expirar")
	}
}

// TestQueueCollector_ErroOmiteAmostraEContaErro: fonte que falha NÃO reporta um
// zero falso — a amostra some e metrics_coleta_erros_total{fonte} sobe; a outra
// fonte segue.
func TestQueueCollector_ErroOmiteAmostraEContaErro(t *testing.T) {
	q := &fakeQueues{patientsErr: errors.New("db fora"), appts: 4}
	o := New(Config{Queues: q, Clock: &stepClock{t: time.Unix(1_700_000_000, 0)}})
	out := scrape(t, o)

	if strings.Contains(out, "psicoman_fila_pacientes_pendentes ") {
		t.Error("fonte com erro deve omitir a amostra (nada de zero falso)")
	}
	if !strings.Contains(out, `psicoman_metrics_coleta_erros_total{fonte="pacientes_pendentes"} 1`) {
		t.Error("o erro da coleta deveria ser contado por fonte")
	}
	if !strings.Contains(out, "psicoman_fila_agendamentos_pendentes 4") {
		t.Error("a outra fonte deveria seguir funcionando")
	}
}

func TestObs_SemQueuesOmiteFamiliasDeFila(t *testing.T) {
	if strings.Contains(scrape(t, newObs(false)), "psicoman_fila_") {
		t.Error("sem QueueStats as famílias de fila devem ser omitidas")
	}
}

// --- decoradores ---

type fakeMetrics struct {
	app.NopMetrics
	mu      sync.Mutex
	cipher  []string
	ai      []string
	calls   []string
	elapsed []time.Duration
}

func (f *fakeMetrics) CipherOp(op app.CipherOp, r app.Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cipher = append(f.cipher, string(op)+":"+string(r))
}
func (f *fakeMetrics) AIRequest(r app.Result, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ai = append(f.ai, string(r))
	f.elapsed = append(f.elapsed, d)
}
func (f *fakeMetrics) CalendarCall(op app.CalendarOp, r app.Result, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, string(op)+":"+string(r))
	f.elapsed = append(f.elapsed, d)
}

type stubCipher struct{ err error }

func (s stubCipher) Seal(_ context.Context, p string) (string, error) { return "env:" + p, s.err }
func (s stubCipher) Open(_ context.Context, e string) (string, error) {
	return strings.TrimPrefix(e, "env:"), s.err
}

func TestWrapCipher_ContaEPreservaValorEErro(t *testing.T) {
	m := &fakeMetrics{}
	ok := WrapCipher(stubCipher{}, m)
	env, err := ok.Seal(context.Background(), "claro")
	if err != nil || env != "env:claro" {
		t.Fatalf("valor/erro devem passar intactos: %q %v", env, err)
	}
	if pl, _ := ok.Open(context.Background(), env); pl != "claro" {
		t.Errorf("Open deve devolver o claro, veio %q", pl)
	}
	boom := errors.New("chave errada")
	bad := WrapCipher(stubCipher{err: boom}, m)
	if _, err := bad.Open(context.Background(), "x"); !errors.Is(err, boom) {
		t.Errorf("o erro original deve ser preservado, veio %v", err)
	}
	want := []string{"seal:ok", "open:ok", "open:erro"}
	if strings.Join(m.cipher, ",") != strings.Join(want, ",") {
		t.Errorf("cifra: quero %v, veio %v", want, m.cipher)
	}
}

type stubAI struct{ err error }

func (s stubAI) Summarize(context.Context, app.AISummaryInput) (*app.AISummaryOutput, error) {
	time.Sleep(2 * time.Millisecond)
	if s.err != nil {
		return nil, s.err
	}
	return &app.AISummaryOutput{}, nil
}

func TestWrapAI_ContaLatenciaEPreservaOptIn(t *testing.T) {
	m := &fakeMetrics{}
	if WrapAI(nil, m) != nil {
		t.Fatal("IA desligada (nil) deve continuar nil: o decorador não pode ligar a IA")
	}
	w := WrapAI(stubAI{}, m)
	if _, err := w.Summarize(context.Background(), app.AISummaryInput{}); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("429")
	if _, err := WrapAI(stubAI{err: boom}, m).Summarize(context.Background(), app.AISummaryInput{}); !errors.Is(err, boom) {
		t.Errorf("erro deve passar intacto: %v", err)
	}
	if strings.Join(m.ai, ",") != "ok,erro" {
		t.Errorf("resultados: quero ok,erro; veio %v", m.ai)
	}
	if m.elapsed[0] <= 0 {
		t.Errorf("a latência deveria ser medida, veio %v", m.elapsed[0])
	}
}

type stubChecker struct {
	conflict bool
	err      error
}

func (s stubChecker) HasConflict(context.Context, time.Time, time.Time) (bool, error) {
	return s.conflict, s.err
}

type stubCal struct{ err error }

func (s stubCal) CreateEvent(context.Context, app.CalendarEvent) (*app.CalendarEventResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &app.CalendarEventResult{EventID: "e1"}, nil
}

func TestWrapConflictChecker_OkConflitoErro(t *testing.T) {
	m := &fakeMetrics{}
	now := time.Now()
	livre, _ := WrapConflictChecker(stubChecker{}, m).HasConflict(context.Background(), now, now)
	ocupado, _ := WrapConflictChecker(stubChecker{conflict: true}, m).HasConflict(context.Background(), now, now)
	_, err := WrapConflictChecker(stubChecker{err: errors.New("rede")}, m).HasConflict(context.Background(), now, now)
	if livre || !ocupado || err == nil {
		t.Fatalf("valores/erro devem passar intactos: livre=%v ocupado=%v err=%v", livre, ocupado, err)
	}
	want := "freebusy:ok,freebusy:conflito,freebusy:erro"
	if strings.Join(m.calls, ",") != want {
		t.Errorf("quero %s, veio %v", want, m.calls)
	}
}

func TestWrapSessionCalendar_CreateEvent(t *testing.T) {
	m := &fakeMetrics{}
	res, err := WrapSessionCalendar(stubCal{}, m).CreateEvent(context.Background(), app.CalendarEvent{})
	if err != nil || res.EventID != "e1" {
		t.Fatalf("resultado deve passar intacto: %+v %v", res, err)
	}
	if _, err := WrapSessionCalendar(stubCal{err: errors.New("403")}, m).CreateEvent(context.Background(), app.CalendarEvent{}); err == nil {
		t.Fatal("erro deve passar intacto")
	}
	if strings.Join(m.calls, ",") != "create_event:ok,create_event:erro" {
		t.Errorf("veio %v", m.calls)
	}
}
