package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

// registerPortal cadastra um paciente pelo portal (cria patient pendente +
// credencial patient_user) e devolve o id do paciente.
func registerPortal(t *testing.T, c *apiClient, name, phone, email string) string {
	t.Helper()
	res := c.do(http.MethodPost, "/v1/portal/register",
		`{"name":"`+name+`","phone":"`+phone+`","email":"`+email+`","password":"senha-forte-123"}`, nil)
	if res.status != http.StatusCreated {
		t.Fatalf("registrar no portal: status %d (%s)", res.status, res.body)
	}
	return extractID(res.body)
}

// portalAuthHeader gera um token de sessão REAL para o paciente (auth record
// patient_user vinculado ao email) e devolve o header Authorization: Bearer. É
// a identidade real da Fase 9 (substitui o antigo seam X-Portal-Email).
func portalAuthHeader(t *testing.T, app *tests.TestApp, email string) map[string]string {
	t.Helper()
	rec, err := app.FindFirstRecordByFilter("patient_user", "email = {:e}", map[string]any{"e": email})
	if err != nil {
		t.Fatalf("achar patient_user de %q: %v", email, err)
	}
	token, err := rec.NewAuthToken()
	if err != nil {
		t.Fatalf("gerar token do paciente: %v", err)
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

// approvePatient aprova um paciente pela rota do app (header do Pangolin).
func approvePatient(t *testing.T, c *apiClient, id string) {
	t.Helper()
	res := c.do(http.MethodPost, "/v1/app/patients/"+id+"/approve", "", appHeaders())
	if res.status != http.StatusOK {
		t.Fatalf("aprovar paciente: status %d (%s)", res.status, res.body)
	}
}

// TestPortalGateAprovadoPassa (9.1): paciente aprovado, com sessão REAL →
// rota protegida 2xx.
func TestPortalGateAprovadoPassa(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const email = "aprovado@exemplo.com"
	id := registerPortal(t, c, "Aprovado", "11999990101", email)
	approvePatient(t, c, id)

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", portalAuthHeader(t, app, email))
	if res.status != http.StatusOK {
		t.Fatalf("aprovado: quero 200, veio %d (%s)", res.status, res.body)
	}
}

// TestPortalGatePendente403 (9.1): paciente pendente (sessão real) → 403.
func TestPortalGatePendente403(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const email = "pendente@exemplo.com"
	registerPortal(t, c, "Pendente", "11999990102", email)

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", portalAuthHeader(t, app, email))
	if res.status != http.StatusForbidden {
		t.Fatalf("pendente: quero 403, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"acesso_negado"`) {
		t.Errorf("code esperado acesso_negado: %s", res.body)
	}
}

// TestPortalGateRejeitado403 (9.1): paciente rejeitado (sessão real) → 403.
func TestPortalGateRejeitado403(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const email = "rejeitado@exemplo.com"
	id := registerPortal(t, c, "Rejeitado", "11999990103", email)

	rej := c.do(http.MethodPost, "/v1/app/patients/"+id+"/reject", "", appHeaders())
	if rej.status != http.StatusOK {
		t.Fatalf("reject: quero 200, veio %d (%s)", rej.status, rej.body)
	}

	res := c.do(http.MethodGet, "/v1/portal/sessions", "", portalAuthHeader(t, app, email))
	if res.status != http.StatusForbidden {
		t.Fatalf("rejeitado: quero 403, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"acesso_negado"`) {
		t.Errorf("code esperado acesso_negado: %s", res.body)
	}
}

// TestPortalGateExcecoesLivres (9.1): register (pendente) e me (pendente/
// rejeitado, com sessão real) ficam fora do gate.
func TestPortalGateExcecoesLivres(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const email = "livre@exemplo.com"
	id := registerPortal(t, c, "Livre", "11999990104", email)
	hdr := portalAuthHeader(t, app, email)

	// me com pendente → 200 (vê o próprio estado).
	me := c.do(http.MethodGet, "/v1/portal/me", "", hdr)
	if me.status != http.StatusOK {
		t.Fatalf("me pendente: quero 200, veio %d (%s)", me.status, me.body)
	}

	// me com rejeitado → 200.
	if rej := c.do(http.MethodPost, "/v1/app/patients/"+id+"/reject", "", appHeaders()); rej.status != http.StatusOK {
		t.Fatalf("reject: quero 200, veio %d (%s)", rej.status, rej.body)
	}
	meRej := c.do(http.MethodGet, "/v1/portal/me", "", hdr)
	if meRej.status != http.StatusOK {
		t.Fatalf("me rejeitado: quero 200, veio %d (%s)", meRej.status, meRej.body)
	}
}

// TestPortalGateSemSessao401 (9.1): rota protegida sem sessão → 401.
func TestPortalGateSemSessao401(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/portal/sessions", "", nil)
	if res.status != http.StatusUnauthorized {
		t.Fatalf("sem sessão: quero 401, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"nao_autenticado"`) {
		t.Errorf("code esperado nao_autenticado: %s", res.body)
	}
}

// TestPortalGatePathExato (9.1, Finding 4): a exceção casa só o path canônico.
// GET /v1/portal/me passa; /v1/portal/me/ (barra final) NÃO herda a exceção.
func TestPortalGatePathExato(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const email = "exato@exemplo.com"
	registerPortal(t, c, "Exato", "11999990105", email)
	hdr := portalAuthHeader(t, app, email)

	me := c.do(http.MethodGet, "/v1/portal/me", "", hdr)
	if me.status != http.StatusOK {
		t.Fatalf("me canônico: quero 200, veio %d (%s)", me.status, me.body)
	}

	// me com barra final → não é a exceção; o gate roda e, com pendente, nega 403.
	meSlash := c.do(http.MethodGet, "/v1/portal/me/", "", hdr)
	if meSlash.status == http.StatusOK {
		t.Errorf("/v1/portal/me/ não deveria herdar a exceção (status %d, %s)", meSlash.status, meSlash.body)
	}
}
