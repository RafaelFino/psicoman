package app

import (
	"context"
	"time"
)

// Tipos FECHADOS (enums) dos eventos de negócio. Os valores viram rótulos de
// métrica, então são um conjunto pequeno e fixo: NUNCA um id, e-mail, nome ou
// texto livre (R8.9). Este arquivo não importa o PocketBase nem a lib de métricas
// — a implementação Prometheus vive em internal/platform/metrics (R8.11).

// PatientEvent é um evento do ciclo de vida do paciente.
type PatientEvent string

const (
	PatientRegistered PatientEvent = "cadastrado"
	PatientApproved   PatientEvent = "aprovado"
	PatientRejected   PatientEvent = "recusado"
)

// SessionEvent é um evento do ciclo de vida da sessão.
type SessionEvent string

const (
	SessionCreated   SessionEvent = "criada"
	SessionConfirmed SessionEvent = "confirmada"
	SessionCanceled  SessionEvent = "cancelada"
	SessionFinished  SessionEvent = "finalizada"
	SessionNoShow    SessionEvent = "falta"
)

// ApptEvent é um evento do pedido de agendamento do portal.
type ApptEvent string

const (
	ApptCreated   ApptEvent = "criado"
	ApptConfirmed ApptEvent = "confirmado"
	ApptRefused   ApptEvent = "recusado"
)

// Result é o desfecho de uma chamada a uma integração.
type Result string

const (
	ResultOK       Result = "ok"
	ResultError    Result = "erro"
	ResultConflict Result = "conflito"
)

// CalendarOp é a operação chamada no Google Calendar.
type CalendarOp string

const (
	CalendarFreeBusy    CalendarOp = "freebusy"
	CalendarCreateEvent CalendarOp = "create_event"
)

// CipherOp é a operação da cifra de campo.
type CipherOp string

const (
	CipherSeal CipherOp = "seal"
	CipherOpen CipherOp = "open"
)

// Metrics é a porta de observabilidade de negócio. Os serviços a recebem por
// SetMetrics (default NopMetrics{}), de modo que os testes existentes não
// precisam mudar. Todos os métodos são best-effort e NUNCA devem falhar nem
// bloquear a operação de negócio.
type Metrics interface {
	Patient(ev PatientEvent)
	Session(ev SessionEvent)
	Appointment(ev ApptEvent)
	DebtGenerated()
	PaymentRecorded()
	InvoicePDFGenerated()
	// AuditRecorded/AuditFailed recebem SEMPRE uma constante domain.AuditAction*
	// (enum fechado), nunca um valor vindo de entrada.
	AuditRecorded(action string)
	AuditFailed(action string)
	AIRequest(res Result, d time.Duration)
	CalendarCall(op CalendarOp, res Result, d time.Duration)
	// CalendarDegraded conta um agendamento feito SEM evento por falta de
	// credencial do Google.
	CalendarDegraded()
	CipherOp(op CipherOp, res Result)
}

// NopMetrics implementa Metrics descartando tudo (default dos serviços e dos
// testes).
type NopMetrics struct{}

var _ Metrics = NopMetrics{}

func (NopMetrics) Patient(PatientEvent)                           {}
func (NopMetrics) Session(SessionEvent)                           {}
func (NopMetrics) Appointment(ApptEvent)                          {}
func (NopMetrics) DebtGenerated()                                 {}
func (NopMetrics) PaymentRecorded()                               {}
func (NopMetrics) InvoicePDFGenerated()                           {}
func (NopMetrics) AuditRecorded(string)                           {}
func (NopMetrics) AuditFailed(string)                             {}
func (NopMetrics) AIRequest(Result, time.Duration)                {}
func (NopMetrics) CalendarCall(CalendarOp, Result, time.Duration) {}
func (NopMetrics) CalendarDegraded()                              {}
func (NopMetrics) CipherOp(CipherOp, Result)                      {}

// QueueStats é a porta de leitura das filas de trabalho para os gauges de
// observabilidade (implementada em platform/pb com CountRecords). Chamada pelo
// coletor com timeout e cache; um erro omite a amostra.
type QueueStats interface {
	PendingPatients(ctx context.Context) (int, error)
	PendingAppointments(ctx context.Context) (int, error)
}
