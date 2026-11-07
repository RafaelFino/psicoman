package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
)

// knownAuditActions é o conjunto FECHADO de ações de auditoria aceitas como
// rótulo `acao`. Qualquer valor fora dele vira "outro": o rótulo nunca pode
// carregar texto vindo de entrada (R8.9), mesmo que um chamador erre.
var knownAuditActions = map[string]bool{
	domain.AuditActionLoginSuccess:      true,
	domain.AuditActionLoginFailure:      true,
	domain.AuditActionCreate:            true,
	domain.AuditActionUpdate:            true,
	domain.AuditActionDelete:            true,
	domain.AuditActionRead:              true,
	domain.AuditActionDebtGenerate:      true,
	domain.AuditActionDebtPay:           true,
	domain.AuditActionBackup:            true,
	domain.AuditActionRestore:           true,
	domain.AuditActionConfig:            true,
	domain.AuditActionApprove:           true,
	domain.AuditActionReject:            true,
	domain.AuditActionSessionConfirm:    true,
	domain.AuditActionSessionCancel:     true,
	domain.AuditActionSessionReschedule: true,
	domain.AuditActionAIProcess:         true,
}

func auditLabel(action string) string {
	if knownAuditActions[action] {
		return action
	}
	return "outro"
}

// appMetrics implementa app.Metrics sobre o registry. Todos os rótulos vêm dos
// enums fechados de app (ou de knownAuditActions).
type appMetrics struct {
	patients  *prometheus.CounterVec
	sessions  *prometheus.CounterVec
	appts     *prometheus.CounterVec
	debts     prometheus.Counter
	payments  prometheus.Counter
	pdfs      prometheus.Counter
	auditOK   *prometheus.CounterVec
	auditFail *prometheus.CounterVec

	aiCalls    *prometheus.CounterVec
	aiDuration *prometheus.HistogramVec

	calCalls    *prometheus.CounterVec
	calDuration *prometheus.HistogramVec
	calDegraded prometheus.Counter

	cipher *prometheus.CounterVec
}

var _ app.Metrics = (*appMetrics)(nil)

func newAppMetrics(reg *prometheus.Registry) *appMetrics {
	counter := func(name, help string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: name, Help: help}, labels)
	}
	plain := func(name, help string) prometheus.Counter {
		return prometheus.NewCounter(prometheus.CounterOpts{Namespace: namespace, Name: name, Help: help})
	}
	m := &appMetrics{
		patients:  counter("pacientes_eventos_total", "Eventos do ciclo de vida do paciente.", "evento"),
		sessions:  counter("sessoes_eventos_total", "Eventos do ciclo de vida da sessão.", "evento"),
		appts:     counter("agendamentos_eventos_total", "Eventos dos pedidos de agendamento do portal.", "evento"),
		debts:     plain("debitos_gerados_total", "Débitos gerados (por sessão ou por ciclo)."),
		payments:  plain("pagamentos_registrados_total", "Pagamentos registrados."),
		pdfs:      plain("pdfs_gerados_total", "PDFs de cobrança gerados."),
		auditOK:   counter("audit_log_total", "Entradas gravadas no audit_log por ação.", "acao"),
		auditFail: counter("audit_log_falhas_total", "Falhas ao gravar o audit_log por ação.", "acao"),

		aiCalls: counter("gemini_chamadas_total", "Chamadas ao Gemini por resultado.", "resultado"),
		aiDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "gemini_duracao_segundos",
			Help: "Duração das chamadas ao Gemini.", Buckets: durationBuckets,
		}, []string{"resultado"}),

		calCalls: counter("calendar_chamadas_total", "Chamadas ao Google Calendar por operação e resultado.", "op", "resultado"),
		calDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "calendar_duracao_segundos",
			Help: "Duração das chamadas ao Google Calendar.", Buckets: durationBuckets,
		}, []string{"op"}),
		calDegraded: plain("calendar_degradado_total", "Agendamentos feitos SEM evento por falta de credencial do Google."),

		cipher: counter("cifra_operacoes_total", "Operações da cifra de campo por tipo e resultado.", "op", "resultado"),
	}
	reg.MustRegister(m.patients, m.sessions, m.appts, m.debts, m.payments, m.pdfs,
		m.auditOK, m.auditFail, m.aiCalls, m.aiDuration, m.calCalls, m.calDuration, m.calDegraded, m.cipher)
	return m
}

func (m *appMetrics) Patient(ev app.PatientEvent)  { m.patients.WithLabelValues(string(ev)).Inc() }
func (m *appMetrics) Session(ev app.SessionEvent)  { m.sessions.WithLabelValues(string(ev)).Inc() }
func (m *appMetrics) Appointment(ev app.ApptEvent) { m.appts.WithLabelValues(string(ev)).Inc() }
func (m *appMetrics) DebtGenerated()               { m.debts.Inc() }
func (m *appMetrics) PaymentRecorded()             { m.payments.Inc() }
func (m *appMetrics) InvoicePDFGenerated()         { m.pdfs.Inc() }
func (m *appMetrics) AuditRecorded(action string) {
	m.auditOK.WithLabelValues(auditLabel(action)).Inc()
}
func (m *appMetrics) AuditFailed(action string) {
	m.auditFail.WithLabelValues(auditLabel(action)).Inc()
}
func (m *appMetrics) CalendarDegraded() { m.calDegraded.Inc() }
func (m *appMetrics) CipherOp(op app.CipherOp, r app.Result) {
	m.cipher.WithLabelValues(string(op), string(r)).Inc()
}

func (m *appMetrics) AIRequest(r app.Result, d time.Duration) {
	m.aiCalls.WithLabelValues(string(r)).Inc()
	m.aiDuration.WithLabelValues(string(r)).Observe(d.Seconds())
}

func (m *appMetrics) CalendarCall(op app.CalendarOp, r app.Result, d time.Duration) {
	m.calCalls.WithLabelValues(string(op), string(r)).Inc()
	m.calDuration.WithLabelValues(string(op)).Observe(d.Seconds())
}
