package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

// testClock é um relógio avançável para expirar o state sem esperar o TTL.
type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time { return c.t }

// authorizeFlow executa GET /v1/admin/google/authorize e devolve o state (da
// Location) e o valor do cookie psicoman_oauth_state (do Set-Cookie).
func authorizeFlow(t *testing.T, c *apiClient, extra map[string]string) (state string, cookie *http.Cookie, res apiResp) {
	t.Helper()
	hdr := appHeaders()
	for k, v := range extra {
		hdr[k] = v
	}
	res = c.do(http.MethodGet, "/v1/admin/google/authorize", "", hdr)
	if res.status != http.StatusFound {
		t.Fatalf("authorize: quero 302, veio %d (%s)", res.status, res.body)
	}
	loc, err := url.Parse(res.header.Get("Location"))
	if err != nil {
		t.Fatalf("Location inválida: %v", err)
	}
	state = loc.Query().Get("state")
	if state == "" {
		t.Fatalf("a URL de consentimento deveria carregar o state: %s", loc)
	}
	for _, ck := range (&http.Response{Header: res.header}).Cookies() {
		if ck.Name == "psicoman_oauth_state" {
			cookie = ck
		}
	}
	if cookie == nil {
		t.Fatal("authorize deveria gravar o cookie psicoman_oauth_state")
	}
	return state, cookie, res
}

// callbackWith chama o callback com state na query e (opcionalmente) o cookie.
func callbackWith(c *apiClient, query, cookieValue string) apiResp {
	hdr := appHeaders()
	if cookieValue != "" {
		hdr["Cookie"] = "psicoman_oauth_state=" + cookieValue
	}
	return c.do(http.MethodGet, "/v1/admin/google/callback?"+query, "", hdr)
}

// cookieExpirado confere que a resposta expira o cookie do state (MaxAge<0).
func cookieExpirado(res apiResp) bool {
	for _, ck := range (&http.Response{Header: res.header}).Cookies() {
		if ck.Name == "psicoman_oauth_state" && ck.MaxAge < 0 {
			return true
		}
	}
	return false
}

// autorizarGoogle faz o fluxo completo válido (authorize -> callback) e devolve
// a resposta do callback.
func autorizarGoogle(t *testing.T, c *apiClient) apiResp {
	t.Helper()
	state, cookie, _ := authorizeFlow(t, c, nil)
	return callbackWith(c, "code=fake-code&state="+url.QueryEscape(state), cookie.Value)
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

	// authorize (state + cookie) -> callback com um code qualquer (o fake /token
	// ignora e devolve o refresh).
	cb := autorizarGoogle(t, c)
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

// TestGoogleOAuthAuthorizeCookieAtributos: o cookie do state é HttpOnly,
// SameSite=Lax, Path=/v1/admin/google, MaxAge=600; sem Secure em HTTP puro e
// Secure atrás de proxy HTTPS (X-Forwarded-Proto).
func TestGoogleOAuthAuthorizeCookieAtributos(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "ignored")
	t.Cleanup(google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token"))
	app := newTestApp(t)
	c := newClient(t, app)

	_, ck, _ := authorizeFlow(t, c, nil)
	if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/v1/admin/google" || ck.MaxAge != 600 {
		t.Errorf("atributos do cookie incorretos: %+v", ck)
	}
	if ck.Secure {
		t.Error("em HTTP puro o cookie não deve ser Secure")
	}

	_, ckTLS, _ := authorizeFlow(t, c, map[string]string{"X-Forwarded-Proto": "https"})
	if !ckTLS.Secure {
		t.Error("atrás de proxy HTTPS o cookie deve ser Secure")
	}
}

// TestGoogleOAuthCallbackStateInvalido cobre os caminhos NEGADOS: sem cookie,
// state diferente do cookie, sem state na query, reuso e expirado — todos 400
// state_invalido com a mesma mensagem, SEM trocar o code (token não persiste) e
// expirando o cookie.
func TestGoogleOAuthCallbackStateInvalido(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "1//nao-deve-persistir")
	t.Cleanup(google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token"))

	clk := &testClock{t: time.Unix(1_700_000_000, 0)}
	prev := oauthStateClock
	oauthStateClock = clk
	t.Cleanup(func() { oauthStateClock = prev })

	app := newTestApp(t)
	c := newClient(t, app)

	checa := func(nome string, res apiResp) {
		t.Helper()
		if res.status != http.StatusBadRequest || !strings.Contains(res.body, `"code":"state_invalido"`) {
			t.Errorf("%s: quero 400 state_invalido, veio %d (%s)", nome, res.status, res.body)
		}
		if !strings.Contains(res.body, "inválido ou expirado") {
			t.Errorf("%s: mensagem padrão ausente: %s", nome, res.body)
		}
		if !cookieExpirado(res) {
			t.Errorf("%s: o cookie do state deveria ser expirado", nome)
		}
	}

	// 1) sem cookie.
	state, ck, _ := authorizeFlow(t, c, nil)
	checa("sem cookie", callbackWith(c, "code=x&state="+url.QueryEscape(state), ""))

	// 2) state da query diferente do cookie (o state acima já foi... consumido?
	// não: o 1º falhou antes do Consume, então ainda vale; emitimos outro par).
	state2, ck2, _ := authorizeFlow(t, c, nil)
	checa("state diferente do cookie", callbackWith(c, "code=x&state="+url.QueryEscape(state2), ck.Value))

	// 3) sem state na query.
	checa("sem state na query", callbackWith(c, "code=x", ck2.Value))

	// 4) reuso: o 1º uso válido passa; o 2º com o MESMO par falha.
	if ok := callbackWith(c, "code=x&state="+url.QueryEscape(state2), ck2.Value); ok.status != http.StatusOK {
		t.Fatalf("1º uso válido: quero 200, veio %d (%s)", ok.status, ok.body)
	}
	checa("reuso", callbackWith(c, "code=x&state="+url.QueryEscape(state2), ck2.Value))

	// 5) expirado: avança o relógio do store além do TTL.
	state3, ck3, _ := authorizeFlow(t, c, nil)
	clk.t = clk.t.Add(11 * time.Minute)
	checa("expirado", callbackWith(c, "code=x&state="+url.QueryEscape(state3), ck3.Value))
}

// TestGoogleOAuthCallbackErroDoGoogleExpiraCookie: o `error` do Google (422)
// também expira o cookie.
func TestGoogleOAuthCallbackErroDoGoogleExpiraCookie(t *testing.T) {
	withGoogleOAuthConfig(t)
	srv := fakeOAuthServer(t, "ignored")
	t.Cleanup(google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token"))
	app := newTestApp(t)
	c := newClient(t, app)

	_, ck, _ := authorizeFlow(t, c, nil)
	res := callbackWith(c, "error=access_denied", ck.Value)
	if res.status != http.StatusUnprocessableEntity || !cookieExpirado(res) {
		t.Errorf("error do Google: quero 422 e cookie expirado, veio %d expirado=%v", res.status, cookieExpirado(res))
	}
}
