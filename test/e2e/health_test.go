package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// getPublic faz um GET sem autenticação (as rotas de observabilidade ficam fora
// da auth) e devolve status + corpo.
func getPublic(t *testing.T, base, path string) (int, string) {
	t.Helper()
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestLivez: /v1/livez responde 200 (liveness), sem checar dependências.
func TestLivez(t *testing.T) {
	base := startServer(t)
	status, body := getPublic(t, base, "/v1/livez")
	if status != http.StatusOK {
		t.Fatalf("/livez: quero 200, veio %d (%s)", status, body)
	}
}

// TestReadyzSemGoogle: sem credenciais do Google no ambiente, /v1/readyz fica
// PRONTO (cifra + DB ok) mas reporta o Google como degraded — o Google não
// bloqueia a prontidão (a app opera sem ele). Prova que o /readyz reflete o
// estado real do OAuth (R5.1, dep. Fase 2).
func TestReadyzSemGoogle(t *testing.T) {
	base := startServer(t)
	status, body := getPublic(t, base, "/v1/readyz")
	if status != http.StatusOK {
		t.Fatalf("/readyz: quero 200 (pronto), veio %d (%s)", status, body)
	}
	var env struct {
		Data struct {
			Ready  bool `json:"ready"`
			Checks struct {
				Cipher string `json:"cipher"`
				DB     string `json:"db"`
				Google string `json:"google"`
			} `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decodificar readyz: %v (%s)", err, body)
	}
	if !env.Data.Ready {
		t.Errorf("readyz deveria estar pronto (cifra+DB ok): %s", body)
	}
	if env.Data.Checks.Cipher != "ok" || env.Data.Checks.DB != "ok" {
		t.Errorf("checks cifra/DB deveriam ser ok: %+v", env.Data.Checks)
	}
	if env.Data.Checks.Google != "degraded" {
		t.Errorf("sem Google autorizado, google deveria ser degraded, veio %q", env.Data.Checks.Google)
	}
}
