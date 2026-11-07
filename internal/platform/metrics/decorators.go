package metrics

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain"
)

// Decoradores: instrumentam as integrações SEM tocar os serviços. Cada um só
// mede (resultado + duração) e devolve exatamente o que a integração devolveu —
// nunca altera o valor nem o erro. Os rótulos vêm dos enums fechados de app.

// --- cifra de campo ---

type cipherMetrics struct {
	inner domain.Cipher
	m     app.Metrics
}

// WrapCipher conta seal/open por resultado. Nunca vê o claro nem o envelope
// além de repassá-los.
func WrapCipher(c domain.Cipher, m app.Metrics) domain.Cipher {
	return &cipherMetrics{inner: c, m: m}
}

func (w *cipherMetrics) Seal(ctx context.Context, plaintext string) (string, error) {
	out, err := w.inner.Seal(ctx, plaintext)
	w.m.CipherOp(app.CipherSeal, resultOf(err))
	return out, err
}

func (w *cipherMetrics) Open(ctx context.Context, envelope string) (string, error) {
	out, err := w.inner.Open(ctx, envelope)
	w.m.CipherOp(app.CipherOpen, resultOf(err))
	return out, err
}

// --- Gemini ---

type aiMetrics struct {
	inner app.AISummarizer
	m     app.Metrics
}

// WrapAI conta e cronometra as chamadas ao Gemini. Se s for nil (IA desligada)
// devolve nil — o opt-in da IA (presença da chave) NÃO pode ser alterado pelo
// decorador.
func WrapAI(s app.AISummarizer, m app.Metrics) app.AISummarizer {
	if s == nil {
		return nil
	}
	return &aiMetrics{inner: s, m: m}
}

func (w *aiMetrics) Summarize(ctx context.Context, in app.AISummaryInput) (*app.AISummaryOutput, error) {
	t0 := time.Now()
	out, err := w.inner.Summarize(ctx, in)
	w.m.AIRequest(resultOf(err), time.Since(t0))
	return out, err
}

// --- Calendar ---

type conflictMetrics struct {
	inner app.ConflictChecker
	m     app.Metrics
}

// WrapConflictChecker mede o freebusy: ok, conflito (a agenda estava ocupada) ou
// erro (falha da chamada).
func WrapConflictChecker(c app.ConflictChecker, m app.Metrics) app.ConflictChecker {
	return &conflictMetrics{inner: c, m: m}
}

func (w *conflictMetrics) HasConflict(ctx context.Context, startsAt, endsAt time.Time) (bool, error) {
	t0 := time.Now()
	conflict, err := w.inner.HasConflict(ctx, startsAt, endsAt)
	res := resultOf(err)
	if err == nil && conflict {
		res = app.ResultConflict
	}
	w.m.CalendarCall(app.CalendarFreeBusy, res, time.Since(t0))
	return conflict, err
}

type calendarMetrics struct {
	inner app.SessionCalendar
	m     app.Metrics
}

// WrapSessionCalendar mede a criação de evento (create_event).
func WrapSessionCalendar(c app.SessionCalendar, m app.Metrics) app.SessionCalendar {
	return &calendarMetrics{inner: c, m: m}
}

func (w *calendarMetrics) CreateEvent(ctx context.Context, ev app.CalendarEvent) (*app.CalendarEventResult, error) {
	t0 := time.Now()
	out, err := w.inner.CreateEvent(ctx, ev)
	w.m.CalendarCall(app.CalendarCreateEvent, resultOf(err), time.Since(t0))
	return out, err
}

func resultOf(err error) app.Result {
	if err != nil {
		return app.ResultError
	}
	return app.ResultOK
}
