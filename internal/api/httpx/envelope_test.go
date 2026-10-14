package httpx

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogger captura as chamadas de Error para inspeção.
type captureLogger struct {
	calls []string
}

func (c *captureLogger) Error(msg string, args ...any) {
	var b strings.Builder
	b.WriteString(msg)
	for _, a := range args {
		b.WriteString(" ")
		if s, ok := a.(string); ok {
			b.WriteString(s)
		}
	}
	c.calls = append(c.calls, b.String())
}

func TestRespondErrorLogsInternalOn5xx(t *testing.T) {
	cap := &captureLogger{}
	SetLogger(cap)
	defer SetLogger(nil)

	internal := errors.New("falha no banco: coluna x inexistente")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/admin/sessions", nil)

	RespondError(w, r, ErrInternal(internal))

	if w.Code != 500 {
		t.Fatalf("status = %d, quer 500", w.Code)
	}
	// O detalhe técnico foi logado…
	if len(cap.calls) != 1 {
		t.Fatalf("logs = %d, quer 1", len(cap.calls))
	}
	if !strings.Contains(cap.calls[0], "coluna x inexistente") {
		t.Errorf("log não contém o detalhe interno: %q", cap.calls[0])
	}
	// …mas nunca vaza no corpo da resposta.
	body := w.Body.String()
	if strings.Contains(body, "coluna x inexistente") {
		t.Errorf("detalhe interno vazou no corpo: %s", body)
	}
	if !strings.Contains(body, "erro_interno") {
		t.Errorf("corpo sem o código erro_interno: %s", body)
	}
}

func TestRespondErrorDoesNotLog4xx(t *testing.T) {
	cap := &captureLogger{}
	SetLogger(cap)
	defer SetLogger(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x", nil)
	RespondError(w, r, ErrValidation("informe o autor"))

	if w.Code != 422 {
		t.Fatalf("status = %d, quer 422", w.Code)
	}
	if len(cap.calls) != 0 {
		t.Errorf("4xx não deveria logar como erro interno; logs = %d", len(cap.calls))
	}
}

func TestRespondErrorNilLoggerSafe(t *testing.T) {
	SetLogger(nil) // sem logger configurado
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	// Não deve entrar em panic sem logger.
	RespondError(w, r, ErrInternal(errors.New("boom")))
	if w.Code != 500 {
		t.Fatalf("status = %d, quer 500", w.Code)
	}
}
