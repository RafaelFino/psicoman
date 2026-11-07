package app

import (
	"testing"
	"time"
)

// TestNopMetrics_NaoPanica: NopMetrics aceita todos os eventos sem efeito.
func TestNopMetrics_NaoPanica(t *testing.T) {
	var m Metrics = NopMetrics{}
	m.Patient(PatientRegistered)
	m.Session(SessionFinished)
	m.Appointment(ApptConfirmed)
	m.DebtGenerated()
	m.PaymentRecorded()
	m.InvoicePDFGenerated()
	m.AuditRecorded("ler")
	m.AuditFailed("ler")
	m.AIRequest(ResultOK, time.Second)
	m.CalendarCall(CalendarFreeBusy, ResultConflict, time.Millisecond)
	m.CalendarDegraded()
	m.CipherOp(CipherSeal, ResultError)
}

// TestMetricsEnums_ValoresFechadosSemIdentificadores: os valores dos enums são
// os rótulos expostos; nenhum pode carregar caractere de identificador (espaço,
// arroba, barra) — guarda contra um id/e-mail virar rótulo (R8.9).
func TestMetricsEnums_ValoresFechadosSemIdentificadores(t *testing.T) {
	valores := []string{
		string(PatientRegistered), string(PatientApproved), string(PatientRejected),
		string(SessionCreated), string(SessionConfirmed), string(SessionCanceled), string(SessionFinished), string(SessionNoShow),
		string(ApptCreated), string(ApptConfirmed), string(ApptRefused),
		string(ResultOK), string(ResultError), string(ResultConflict),
		string(CalendarFreeBusy), string(CalendarCreateEvent),
		string(CipherSeal), string(CipherOpen),
	}
	for _, v := range valores {
		if v == "" {
			t.Error("enum vazio")
		}
		for _, r := range v {
			if (r < 'a' || r > 'z') && r != '_' {
				t.Errorf("valor de enum %q deve ser [a-z_]+ (rótulo fechado)", v)
				break
			}
		}
	}
}
