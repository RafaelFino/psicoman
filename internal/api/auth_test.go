package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestAppSemHeaderPangolin401 (5.1): /v1/app/* sem o header do Pangolin → 401
// nao_autenticado.
func TestAppSemHeaderPangolin401(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/app/patients/pending", "", nil)
	if res.status != http.StatusUnauthorized {
		t.Fatalf("sem header: quero 401, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"nao_autenticado"`) {
		t.Errorf("code esperado nao_autenticado: %s", res.body)
	}
}

// TestAppComHeaderPangolin2xx (5.1): /v1/app/* com o header → passa (200).
func TestAppComHeaderPangolin2xx(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/app/patients/pending", "", appHeaders())
	if res.status != http.StatusOK {
		t.Fatalf("com header: quero 200, veio %d (%s)", res.status, res.body)
	}
}

// TestAdminSemHeaderPangolin401 (5.1): /v1/admin/* sem o header → 401.
func TestAdminSemHeaderPangolin401(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodPost, "/v1/admin/backup", "", nil)
	if res.status != http.StatusUnauthorized {
		t.Fatalf("admin sem header: quero 401, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"nao_autenticado"`) {
		t.Errorf("code esperado nao_autenticado: %s", res.body)
	}
}

// TestAdminComHeaderPangolin2xx (5.1): /v1/admin/* com o header → 202.
func TestAdminComHeaderPangolin2xx(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders())
	if res.status != http.StatusAccepted {
		t.Fatalf("admin com header: quero 202, veio %d (%s)", res.status, res.body)
	}
}

// TestHealthForaDaAuth (5.1): /v1/health continua 200 sem header (fora dos
// grupos por ator).
func TestHealthForaDaAuth(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/health", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("health: quero 200 sem header, veio %d (%s)", res.status, res.body)
	}
}

// TestRESTAutomaticaContinua403 (5.1): /api/* do PocketBase NÃO recebe o
// middleware do Pangolin — continua 403 por regra nil (não 401). Garante que
// não quebramos o Dashboard nativo nem a REST automática.
func TestRESTAutomaticaContinua403(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/api/collections/note/records", "", nil)
	if res.status != http.StatusForbidden {
		t.Fatalf("REST /api/*: quero 403 (regra nil), veio %d (%s)", res.status, res.body)
	}
}
