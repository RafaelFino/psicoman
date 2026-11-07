package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// recMetrics é um app.Metrics que registra tudo em memória, para provar que cada
// serviço conta os eventos certos nos caminhos de sucesso (e só neles).
type recMetrics struct {
	mu       sync.Mutex
	patients []PatientEvent
	sessions []SessionEvent
	appts    []ApptEvent
	debts    int
	payments int
	pdfs     int
	auditOK  []string
	auditBad []string
	ai       []Result
	calls    []string // "op:resultado"
	degraded int
	cipher   []string // "op:resultado"
}

func (r *recMetrics) Patient(ev PatientEvent) { r.lock(func() { r.patients = append(r.patients, ev) }) }
func (r *recMetrics) Session(ev SessionEvent) { r.lock(func() { r.sessions = append(r.sessions, ev) }) }
func (r *recMetrics) Appointment(ev ApptEvent) {
	r.lock(func() { r.appts = append(r.appts, ev) })
}
func (r *recMetrics) DebtGenerated()       { r.lock(func() { r.debts++ }) }
func (r *recMetrics) PaymentRecorded()     { r.lock(func() { r.payments++ }) }
func (r *recMetrics) InvoicePDFGenerated() { r.lock(func() { r.pdfs++ }) }
func (r *recMetrics) AuditRecorded(a string) {
	r.lock(func() { r.auditOK = append(r.auditOK, a) })
}
func (r *recMetrics) AuditFailed(a string) {
	r.lock(func() { r.auditBad = append(r.auditBad, a) })
}
func (r *recMetrics) AIRequest(res Result, _ time.Duration) {
	r.lock(func() { r.ai = append(r.ai, res) })
}
func (r *recMetrics) CalendarCall(op CalendarOp, res Result, _ time.Duration) {
	r.lock(func() { r.calls = append(r.calls, string(op)+":"+string(res)) })
}
func (r *recMetrics) CalendarDegraded() { r.lock(func() { r.degraded++ }) }
func (r *recMetrics) CipherOp(op CipherOp, res Result) {
	r.lock(func() { r.cipher = append(r.cipher, string(op)+":"+string(res)) })
}
func (r *recMetrics) lock(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn()
}

var _ Metrics = (*recMetrics)(nil)

// --- paciente ---

func TestMetrics_Paciente_CadastroAprovacaoRecusa(t *testing.T) {
	ctx := context.Background()
	repo := newPatientRepo()
	m := &recMetrics{}
	svc := newPatientSvc(repo, fixedAt(2026, 3, 10))
	svc.SetMetrics(m)

	// Cadastro pelo admin conta como cadastrado.
	if _, err := svc.Create(ctx, CreateInput{Name: "Ana", Phone: "(11) 99999-0001", Email: "ana@x.com"}); err != nil {
		t.Fatal(err)
	}
	// Auto-cadastro NOVO pelo portal conta; vínculo a existente NÃO.
	if _, err := svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "Beto", Phone: "11988887777", Email: "beto@x.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterFromPortal(ctx, PortalRegisterInput{Name: "Ana 2", Phone: "11988887766", Email: "ana@x.com"}); err != nil {
		t.Fatal(err)
	}
	if len(m.patients) != 2 {
		t.Fatalf("esperava 2 cadastros (admin + portal novo), veio %v", m.patients)
	}

	// Aprovação: só a transição real conta; repetir é no-op idempotente.
	beto, _ := repo.GetByEmail(ctx, "beto@x.com")
	if _, err := svc.Approve(ctx, beto.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(ctx, beto.ID); err != nil {
		t.Fatal(err)
	}
	repo.byID["pend"] = &patient.Patient{ID: "pend", Name: "Caio", Email: "caio@x.com", ApprovalStatus: patient.PatientPending}
	if _, err := svc.Reject(ctx, "pend"); err != nil {
		t.Fatal(err)
	}

	var aprov, recus int
	for _, ev := range m.patients {
		switch ev {
		case PatientApproved:
			aprov++
		case PatientRejected:
			recus++
		}
	}
	if aprov != 1 || recus != 1 {
		t.Errorf("aprovado=1 e recusado=1 esperados (idempotente não conta), veio aprovado=%d recusado=%d", aprov, recus)
	}
}

func TestMetrics_Paciente_ValidacaoNaoConta(t *testing.T) {
	m := &recMetrics{}
	svc := newPatientSvc(newPatientRepo(), fixedAt(2026, 3, 10))
	svc.SetMetrics(m)
	if _, err := svc.Create(context.Background(), CreateInput{Name: "", Email: "x"}); err == nil {
		t.Fatal("esperava erro de validação")
	}
	if len(m.patients) != 0 {
		t.Errorf("falha não pode contar evento: %v", m.patients)
	}
}

// --- sessão ---

func TestMetrics_Sessao_CicloDeVida(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	m := &recMetrics{}
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 1))
	svc.SetMetrics(m)

	mk := func() *scheduling.Session {
		s, err := svc.Create(ctx, createInput())
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	a := mk()
	if _, err := svc.Confirm(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finish(ctx, a.ID, FinishInput{}); err != nil {
		t.Fatalf("finish: %v", err)
	}
	b := mk()
	if _, err := svc.Cancel(ctx, b.ID, scheduling.CancelByTherapist, "motivo"); err != nil {
		t.Fatal(err)
	}
	c := mk()
	if _, err := svc.Confirm(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkNoShow(ctx, c.ID); err != nil {
		t.Fatalf("no-show: %v", err)
	}

	cont := map[SessionEvent]int{}
	for _, ev := range m.sessions {
		cont[ev]++
	}
	want := map[SessionEvent]int{
		SessionCreated: 3, SessionConfirmed: 2, SessionFinished: 1, SessionCanceled: 1, SessionNoShow: 1,
	}
	for ev, n := range want {
		if cont[ev] != n {
			t.Errorf("evento %s: quero %d, veio %d (todos: %v)", ev, n, cont[ev], m.sessions)
		}
	}
}

func TestMetrics_Sessao_FalhaNaoConta(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	m := &recMetrics{}
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 1))
	svc.SetMetrics(m)

	// Conflito de agenda: nada é criado nem contado.
	svc.SetConflictChecker(&fakeConflictChecker{conflict: true})
	if _, err := svc.Create(ctx, createInput()); err == nil {
		t.Fatal("esperava conflito")
	}
	// Sessão inexistente.
	if _, err := svc.Confirm(ctx, "nao-existe"); err == nil {
		t.Fatal("esperava erro")
	}
	if len(m.sessions) != 0 {
		t.Errorf("falhas não podem contar eventos: %v", m.sessions)
	}
}

// TestMetrics_Sessao_CalendarDegradado: agendar SEM Calendar configurado conta
// calendar_degradado; com Calendar, não.
func TestMetrics_Sessao_CalendarDegradado(t *testing.T) {
	ctx := context.Background()
	patients := newPatientRepo()
	seedPatient(patients, "p1")

	m := &recMetrics{}
	svc := newSessionSvc(newSessionRepo(), patients, fixedAt(2026, 3, 1))
	svc.SetMetrics(m)
	if _, err := svc.Create(ctx, createInput()); err != nil {
		t.Fatal(err)
	}
	if m.degraded != 1 {
		t.Errorf("sem Calendar: degraded=1 esperado, veio %d", m.degraded)
	}

	m2 := &recMetrics{}
	svc2 := newSessionSvc(newSessionRepo(), patients, fixedAt(2026, 3, 1))
	svc2.SetMetrics(m2)
	svc2.SetCalendar(&fakeCalendar{result: CalendarEventResult{EventID: "e"}}, nil)
	if _, err := svc2.Create(ctx, createInput()); err != nil {
		t.Fatal(err)
	}
	if m2.degraded != 0 {
		t.Errorf("com Calendar: degraded=0 esperado, veio %d", m2.degraded)
	}
}

// --- pedido de agendamento ---

func TestMetrics_Pedido_CriaConfirmaRecusa(t *testing.T) {
	ctx := context.Background()
	h := newApptHarness(t)
	m := &recMetrics{}
	h.svc.SetMetrics(m)
	s, e := slot()

	a1, err := h.svc.Request(ctx, "ok", AppointmentInput{LocationID: "loc1", SlotStart: s, SlotEnd: e})
	if err != nil {
		t.Fatal(err)
	}
	a2, err := h.svc.Request(ctx, "ok", AppointmentInput{LocationID: "loc1", SlotStart: s.Add(48 * time.Hour), SlotEnd: e.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Confirm(ctx, a1.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Refuse(ctx, a2.ID); err != nil {
		t.Fatal(err)
	}
	// Rejeição (paciente pendente) e reconfirmar não contam.
	if _, err := h.svc.Request(ctx, "pend", AppointmentInput{SlotStart: s, SlotEnd: e}); err == nil {
		t.Fatal("paciente pendente deveria ser recusado")
	}
	if _, err := h.svc.Confirm(ctx, a1.ID); err == nil {
		t.Fatal("reconfirmar deveria falhar")
	}

	want := []ApptEvent{ApptCreated, ApptCreated, ApptConfirmed, ApptRefused}
	if len(m.appts) != len(want) {
		t.Fatalf("eventos esperados %v, veio %v", want, m.appts)
	}
	for i := range want {
		if m.appts[i] != want[i] {
			t.Errorf("evento %d: quero %s, veio %s", i, want[i], m.appts[i])
		}
	}
}

// --- débito, pagamento, PDF ---

func TestMetrics_Debito_SoQuandoCria(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	plans := &fakePlanRepo{active: &billing.Plan{ID: "plan1", PatientID: "p1", Type: billing.PlanPorConsulta, Amount: 15000}}
	svc, _ := newBilling(debts, plans, fixedAt(2026, 3, 10))
	m := &recMetrics{}
	svc.SetMetrics(m)

	sess := finishedSession("p1", "s1", true)
	_ = svc.OnSessionFinished(ctx, sess)
	_ = svc.OnSessionFinished(ctx, sess)                               // idempotente: não duplica nem recontabiliza
	_ = svc.OnSessionFinished(ctx, finishedSession("p1", "s2", false)) // opt-out
	if m.debts != 1 {
		t.Errorf("debitos_gerados: quero 1, veio %d", m.debts)
	}
}

func TestMetrics_Pagamento_SoEmSucesso(t *testing.T) {
	ctx := context.Background()
	debts := newDebtRepo()
	debts.byID["d1"] = &billing.Debt{ID: "d1", PatientID: "p1", Amount: 10000, Status: billing.DebtAberto}
	svc := newPayment(newPaymentRepo(), debts, fixedAt(2026, 3, 10))
	m := &recMetrics{}
	svc.SetMetrics(m)

	if _, _, err := svc.Pay(ctx, "d1", PayInput{Amount: 4000, Method: "pix"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Pay(ctx, "d1", PayInput{Amount: 0, Method: "pix"}); err == nil {
		t.Fatal("valor zero deveria falhar")
	}
	if _, _, err := svc.Pay(ctx, "inexistente", PayInput{Amount: 100}); err == nil {
		t.Fatal("débito inexistente deveria falhar")
	}
	if m.payments != 1 {
		t.Errorf("pagamentos_registrados: quero 1, veio %d", m.payments)
	}
}

func TestMetrics_PDF_GeraEReaproveita(t *testing.T) {
	ctx := context.Background()
	svc, debts, _, _ := newInvoiceSvc()
	m := &recMetrics{}
	svc.SetMetrics(m)
	seedDebt(debts, &billing.Debt{
		ID: "d1", PatientID: "p1", Amount: 15000, Status: billing.DebtAberto,
		DueDate: time.Date(2026, 3, 20, 0, 0, 0, 0, clock.Location()),
	})

	if _, err := svc.GeneratePDF(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GeneratePDF(ctx, "d1"); err != nil { // mesmo conteúdo: reaproveita
		t.Fatal(err)
	}
	if _, err := svc.GeneratePDF(ctx, "inexistente"); err == nil {
		t.Fatal("débito inexistente deveria falhar")
	}
	if m.pdfs != 2 {
		t.Errorf("pdfs_gerados: quero 2 (novo + reaproveitado), veio %d", m.pdfs)
	}
}

// SetMetrics(nil) volta ao no-op: nunca panica.
func TestSetMetrics_NilVoltaAoNop(t *testing.T) {
	ctx := context.Background()
	svc := newPatientSvc(newPatientRepo(), fixedAt(2026, 3, 10))
	svc.SetMetrics(nil)
	if _, err := svc.Create(ctx, CreateInput{Name: "Ana", Phone: "11999990001", Email: "ana@x.com"}); err != nil {
		t.Fatal(err)
	}
}
