package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
)

// withGoogleOAuthConfig troca o configLoader por um determinístico que fornece
// as credenciais do app Google (client_id/secret), para exercitar o fluxo de
// autorização em processo. Restaura no fim.
func withGoogleOAuthConfig(t *testing.T) {
	t.Helper()
	prev := configLoader
	configLoader = func() (config.Config, []config.OriginEvent, error) {
		return config.Config{
			Google: config.GoogleConfig{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
		}, nil, nil
	}
	t.Cleanup(func() { configLoader = prev })
}

// fakeOAuthServer sobe um servidor OAuth FAKE: /authorize (não usado na troca) e
// /token (devolve um refresh token). Nenhuma chamada ao Google real.
func fakeOAuthServer(t *testing.T, refreshToken string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-access","token_type":"Bearer","refresh_token":"` +
			refreshToken + `","expires_in":3600}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestGoogleOAuthAuthorizeRedireciona: GET /v1/admin/google/authorize monta a
// URL de consentimento e redireciona (302), pedindo access_type=offline +
// prompt=consent.
func TestGoogleOAuthAuthorizeRedireciona(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "ignored")
	restore := google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token")
	t.Cleanup(restore)

	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/admin/google/authorize", "", appHeaders())
	if res.status != http.StatusFound {
		t.Fatalf("authorize: quero 302, veio %d (%s)", res.status, res.body)
	}
}

// TestGoogleOAuthCallbackPersisteToken: o callback troca o code pelo refresh
// token (via servidor OAuth fake) e persiste. Depois /status vira ok.
func TestGoogleOAuthCallbackPersisteToken(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "1//fake-refresh-token-xyz")
	restore := google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token")
	t.Cleanup(restore)

	app := newTestApp(t)
	c := newClient(t, app)

	// Antes da autorização: status degraded.
	st := c.do(http.MethodGet, "/v1/admin/google/status", "", appHeaders())
	if st.status != http.StatusOK || !strings.Contains(st.body, `"degraded"`) {
		t.Fatalf("status antes: quero degraded, veio %d (%s)", st.status, st.body)
	}

	// Callback com um code qualquer (o fake /token ignora e devolve o refresh).
	cb := c.do(http.MethodGet, "/v1/admin/google/callback?code=fake-code&state=abc", "", appHeaders())
	if cb.status != http.StatusOK {
		t.Fatalf("callback: quero 200, veio %d (%s)", cb.status, cb.body)
	}

	// Depois: status ok, e o refresh token NÃO aparece em nenhuma resposta.
	st = c.do(http.MethodGet, "/v1/admin/google/status", "", appHeaders())
	if st.status != http.StatusOK || !strings.Contains(st.body, `"ok"`) {
		t.Fatalf("status depois: quero ok, veio %d (%s)", st.status, st.body)
	}
	if strings.Contains(cb.body, "1//fake-refresh-token-xyz") || strings.Contains(st.body, "1//fake-refresh-token-xyz") {
		t.Error("VAZAMENTO: o refresh token apareceu numa resposta da API")
	}
}

// TestGoogleOAuthCallbackErroDoGoogle: ?error=access_denied → 422.
func TestGoogleOAuthCallbackErroDoGoogle(t *testing.T) {
	withGoogleOAuthConfig(t)
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/admin/google/callback?error=access_denied", "", appHeaders())
	if res.status != http.StatusUnprocessableEntity {
		t.Fatalf("callback com erro: quero 422, veio %d (%s)", res.status, res.body)
	}
}

// TestGoogleOAuthAuthorizeSemConfig422: sem client_id/secret, authorize → 422.
func TestGoogleOAuthAuthorizeSemConfig422(t *testing.T) {
	// configLoader default (sem google.*): GoogleOAuth.Configured() == false.
	prev := configLoader
	configLoader = func() (config.Config, []config.OriginEvent, error) {
		return config.Config{}, nil, nil
	}
	t.Cleanup(func() { configLoader = prev })

	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/admin/google/authorize", "", appHeaders())
	if res.status != http.StatusUnprocessableEntity {
		t.Fatalf("authorize sem config: quero 422, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, "google_nao_configurado") {
		t.Errorf("code esperado google_nao_configurado: %s", res.body)
	}
}
