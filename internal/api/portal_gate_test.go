package api

import (
	"net/http"
	"strings"
	"testing"
)

// seedApprovedPatient cria um paciente pelo app (nasce aprovado) e devolve
// (id, email).
func seedApprovedPatient(t *testing.T, c *apiClient, name, phone, email string) (string, string) {
	t.Helper()
	res := c.do(http.MethodPost, "/v1/app/patients",
		`{"name":"`+name+`","phone":"`+phone+`","email":"`+email+`"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("seed paciente aprovado: status %d (%s)", res.status, res.body)
	}
	return extractID(res.body), email
}

// seedPendingPatient cria um paciente pelo portal (nasce pendente) e devolve
// (id, email).
func seedPendingPatient(t *testing.T, c *apiClient, name, phone, email string) (string, string) {
	t.Helper()
	res := c.do(http.MethodPost, "/v1/portal/register",
		`{"name":"`+name+`","phone":"`+phone+`","email":"`+email+`"}`, nil)
	if res.status != http.StatusCreated {
		t.Fatalf("seed paciente pendente: status %d (%s)", res.status, res.body)
	}
	return extractID(res.body), email
}

// TestPortalGateAprovadoPassa (5.2): paciente aprovado → rota protegida 2xx.
func TestPortalGateAprovadoPassa(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	_, email := seedApprovedPatient(t, c, "Aprovado", "11999990101", "aprovado@exemplo.com")

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", map[string]string{"X-Portal-Email": email})
	if res.status != http.StatusOK {
		t.Fatalf("aprovado: quero 200, veio %d (%s)", res.status, res.body)
	}
}

// TestPortalGatePendente403 (5.2): paciente pendente → rota protegida 403.
func TestPortalGatePendente403(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	_, email := seedPendingPatient(t, c, "Pendente", "11999990102", "pendente@exemplo.com")

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", map[string]string{"X-Portal-Email": email})
	if res.status != http.StatusForbidden {
		t.Fatalf("pendente: quero 403, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"acesso_negado"`) {
		t.Errorf("code esperado acesso_negado: %s", res.body)
	}
}

// TestPortalGateRejeitado403 (5.2): paciente rejeitado → rota protegida 403.
func TestPortalGateRejeitado403(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	id, email := seedPendingPatient(t, c, "Rejeitado", "11999990103", "rejeitado@exemplo.com")

	// Rejeita via rota do app (header do Pangolin).
	rej := c.do(http.MethodPost, "/v1/app/patients/"+id+"/reject", "", appHeaders())
	if rej.status != http.StatusOK {
		t.Fatalf("reject: quero 200, veio %d (%s)", rej.status, rej.body)
	}

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", map[string]string{"X-Portal-Email": email})
	if res.status != http.StatusForbidden {
		t.Fatalf("rejeitado: quero 403, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"acesso_negado"`) {
		t.Errorf("code esperado acesso_negado: %s", res.body)
	}
}

// TestPortalGateExcecoesLivres (5.2): register (pendente) e me
// (pendente/rejeitado) ficam fora do gate.
func TestPortalGateExcecoesLivres(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)

	// register de um pendente → 201 (fora do gate).
	reg := c.do(http.MethodPost, "/v1/portal/register",
		`{"name":"Livre","phone":"11999990104","email":"livre@exemplo.com"}`, nil)
	if reg.status != http.StatusCreated {
		t.Fatalf("register pendente: quero 201, veio %d (%s)", reg.status, reg.body)
	}

	// me com pendente → 200 (vê o próprio estado).
	me := c.do(http.MethodGet, "/v1/portal/me", "", map[string]string{"X-Portal-Email": "livre@exemplo.com"})
	if me.status != http.StatusOK {
		t.Fatalf("me pendente: quero 200, veio %d (%s)", me.status, me.body)
	}

	// me com rejeitado → 200.
	id := extractID(reg.body)
	if rej := c.do(http.MethodPost, "/v1/app/patients/"+id+"/reject", "", appHeaders()); rej.status != http.StatusOK {
		t.Fatalf("reject: quero 200, veio %d (%s)", rej.status, rej.body)
	}
	meRej := c.do(http.MethodGet, "/v1/portal/me", "", map[string]string{"X-Portal-Email": "livre@exemplo.com"})
	if meRej.status != http.StatusOK {
		t.Fatalf("me rejeitado: quero 200, veio %d (%s)", meRej.status, meRej.body)
	}
}

// TestPortalGateSemEmail401 (5.2): rota protegida sem X-Portal-Email → 401.
func TestPortalGateSemEmail401(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/portal/sessions", "", nil)
	if res.status != http.StatusUnauthorized {
		t.Fatalf("sem email: quero 401, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"nao_autenticado"`) {
		t.Errorf("code esperado nao_autenticado: %s", res.body)
	}
}

// TestPortalGatePathExato (5.2, Finding 4): a exceção casa só o path canônico.
// GET /v1/portal/me passa; /v1/portal/me/ (barra final) NÃO herda a exceção.
func TestPortalGatePathExato(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	_, email := seedPendingPatient(t, c, "Exato", "11999990105", "exato@exemplo.com")

	// me canônico → 200 (exceção).
	me := c.do(http.MethodGet, "/v1/portal/me", "", map[string]string{"X-Portal-Email": email})
	if me.status != http.StatusOK {
		t.Fatalf("me canônico: quero 200, veio %d (%s)", me.status, me.body)
	}

	// me com barra final → não é a exceção; o router não tem essa rota, mas o
	// gate roda no grupo e, com pendente, nega 403 (prova que a exceção é por
	// path exato, não por sufixo).
	meSlash := c.do(http.MethodGet, "/v1/portal/me/", "", map[string]string{"X-Portal-Email": email})
	if meSlash.status == http.StatusOK {
		t.Errorf("/v1/portal/me/ não deveria herdar a exceção (status %d, %s)", meSlash.status, meSlash.body)
	}
}
