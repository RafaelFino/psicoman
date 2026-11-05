package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/RafaelFino/psicoman/internal/integration/google"
)

// TestLivezInProcess: /v1/livez → 200 no envelope.
func TestLivezInProcess(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/livez", "", nil)
	if res.status != http.StatusOK || !strings.Contains(res.body, `"alive"`) {
		t.Fatalf("/livez: quero 200 alive, veio %d (%s)", res.status, res.body)
	}
}

// TestReadyzDegradedSemGoogle: sem token do Google, /readyz fica pronto (cifra+
// DB ok) e reporta google=degraded.
func TestReadyzDegradedSemGoogle(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/readyz", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("/readyz: quero 200, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"ready":true`) {
		t.Errorf("readyz deveria estar pronto: %s", res.body)
	}
	if !strings.Contains(res.body, `"cipher":"ok"`) || !strings.Contains(res.body, `"db":"ok"`) {
		t.Errorf("cifra/DB deveriam ser ok: %s", res.body)
	}
	if !strings.Contains(res.body, `"google":"degraded"`) {
		t.Errorf("sem Google, google deveria ser degraded: %s", res.body)
	}
}

// TestReadyzGoogleOKAposAutorizacao: após autorizar o Google (callback com
// servidor OAuth fake), /readyz passa a reportar google=ok. Prova que o
// /readyz reflete o estado real do OAuth (R5.1 × Fase 2).
func TestReadyzGoogleOKAposAutorizacao(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "1//fake-refresh-readyz")
	restore := google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token")
	t.Cleanup(restore)

	app := newTestApp(t)
	c := newClient(t, app)

	// Antes: degraded.
	before := c.do(http.MethodGet, "/v1/readyz", "", nil)
	if !strings.Contains(before.body, `"google":"degraded"`) {
		t.Fatalf("antes da autorização: google deveria ser degraded: %s", before.body)
	}

	// Autoriza (callback persiste o token).
	cb := c.do(http.MethodGet, "/v1/admin/google/callback?code=fake&state=s", "", appHeaders())
	if cb.status != http.StatusOK {
		t.Fatalf("callback: quero 200, veio %d (%s)", cb.status, cb.body)
	}

	// Depois: google=ok. (mesmo processo, mesmo GoogleAuthService em memória.)
	after := c.do(http.MethodGet, "/v1/readyz", "", nil)
	if !strings.Contains(after.body, `"google":"ok"`) {
		t.Errorf("após autorização: google deveria ser ok: %s", after.body)
	}
	// O refresh token NUNCA aparece no readyz.
	if strings.Contains(after.body, "1//fake-refresh-readyz") {
		t.Error("VAZAMENTO: refresh token no corpo do readyz")
	}
}
