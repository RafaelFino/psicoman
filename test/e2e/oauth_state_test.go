package e2e

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startServerWithGoogleConfig sobe o binário com um config temporário que traz
// client_id/client_secret FICTÍCIOS (PSICOMAN_CONFIG), para o fluxo
// /v1/admin/google/authorize responder 302. NENHUMA chamada ao Google real: o
// authorize só monta a URL e os callbacks inválidos falham ANTES da troca.
func startServerWithGoogleConfig(t *testing.T) string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "google:\n  client_id: \"e2e-client-id\"\n  client_secret: \"e2e-client-secret\"\n"
	if err := os.WriteFile(cfg, []byte(yaml), 0o600); err != nil {
		t.Fatalf("escrever config: %v", err)
	}

	bin := buildBinary(t)
	addr := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", t.TempDir())
	cmd.Env = append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
		"PSICOMAN_CONFIG="+cfg,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(base + "/v1/health"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("servidor não subiu no prazo")
	return ""
}

// noRedirect devolve um cliente que NÃO segue redirects (queremos o 302 cru).
func noRedirect() *http.Client {
	return &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// getOAuth faz GET com identidade do Pangolin e, opcionalmente, o cookie do state.
func getOAuth(t *testing.T, path, cookie string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Pangolin-User", "terapeuta@exemplo.com")
	if cookie != "" {
		req.Header.Set("Cookie", "psicoman_oauth_state="+cookie)
	}
	resp, err := noRedirect().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// TestOAuthStateAuthorizeECallback (R7.2): o authorize emite o state (na URL do
// consentimento) e o cookie HttpOnly; o callback SEM state, SEM cookie ou com
// state divergente é 400 state_invalido — falha antes de qualquer troca, então
// o Google real não é tocado.
func TestOAuthStateAuthorizeECallback(t *testing.T) {
	base := startServerWithGoogleConfig(t)

	// authorize: 302 + state na Location + cookie psicoman_oauth_state.
	resp := getOAuth(t, base+"/v1/admin/google/authorize", "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize: quero 302, veio %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || loc.Query().Get("state") == "" {
		t.Fatalf("Location deveria carregar o state: %q (%v)", resp.Header.Get("Location"), err)
	}
	state := loc.Query().Get("state")
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "psicoman_oauth_state" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.Value != state {
		t.Fatalf("cookie do state ausente ou incorreto: %+v", cookie)
	}

	// NEGADOS: callback sem state, sem cookie, e com state divergente → 400.
	negados := map[string]*http.Response{
		"sem state":   getOAuth(t, base+"/v1/admin/google/callback?code=x", cookie.Value),
		"sem cookie":  getOAuth(t, base+"/v1/admin/google/callback?code=x&state="+url.QueryEscape(state), ""),
		"divergente":  getOAuth(t, base+"/v1/admin/google/callback?code=x&state=outro-valor", cookie.Value),
		"sem tudo":    getOAuth(t, base+"/v1/admin/google/callback?code=x", ""),
		"state vazio": getOAuth(t, base+"/v1/admin/google/callback?code=x&state=", cookie.Value),
	}
	for nome, r := range negados {
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("callback %s: quero 400, veio %d", nome, r.StatusCode)
		}
	}
	// O corpo traz o code estável state_invalido.
	r := negados["sem state"]
	buf := new(strings.Builder)
	b := make([]byte, 2048)
	n, _ := r.Body.Read(b)
	buf.Write(b[:n])
	if !strings.Contains(buf.String(), `"code":"state_invalido"`) {
		t.Errorf("corpo sem code state_invalido: %s", buf.String())
	}
}
