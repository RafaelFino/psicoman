package portal

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBurstThenBlock(t *testing.T) {
	// 60/min = 1/s, burst 3.
	l := NewRateLimiter(60, 3)
	// 3 primeiras passam (burst).
	for i := 0; i < 3; i++ {
		if !l.allow("ip:1.2.3.4") {
			t.Fatalf("requisição %d deveria passar (burst)", i+1)
		}
	}
	// 4ª estoura.
	if l.allow("ip:1.2.3.4") {
		t.Error("4ª requisição deveria ser bloqueada")
	}
	// Chave diferente não é afetada.
	if !l.allow("ip:5.6.7.8") {
		t.Error("outra chave não deveria estar limitada")
	}
}

func TestRateLimiterEmail(t *testing.T) {
	l := NewRateLimiter(60, 1)
	if !l.AllowEmail("x@y.com") {
		t.Error("primeira por email deveria passar")
	}
	if l.AllowEmail("x@y.com") {
		t.Error("segunda por email deveria bloquear")
	}
	if !l.AllowEmail("") {
		t.Error("email vazio nunca deveria bloquear")
	}
}

func TestSessionIssueVerify(t *testing.T) {
	m := NewSessionManager("segredo-de-teste", time.Hour)
	rec := httptest.NewRecorder()
	m.Issue(rec, "user@example.com")

	// Recupera o cookie e valida.
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	email, err := m.Verify(req)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("email = %q", email)
	}
}

func TestSessionRejectsTampered(t *testing.T) {
	m := NewSessionManager("segredo", time.Hour)
	rec := httptest.NewRecorder()
	m.Issue(rec, "user@example.com")
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		c.Value = c.Value + "x" // adultera
		req.AddCookie(c)
	}
	if _, err := m.Verify(req); err == nil {
		t.Error("sessão adulterada deveria falhar")
	}
}

func TestSessionExpired(t *testing.T) {
	m := NewSessionManager("segredo", -time.Hour) // já expira no passado
	rec := httptest.NewRecorder()
	m.Issue(rec, "user@example.com")
	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	if _, err := m.Verify(req); err == nil {
		t.Error("sessão expirada deveria falhar")
	}
}
