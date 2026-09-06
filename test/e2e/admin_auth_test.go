package e2e

import (
	"net/http"
	"testing"
)

// TestAdminMeAuthenticated valida que /v1/admin/me devolve o terapeuta com
// credenciais válidas.
func TestAdminMeAuthenticated(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.AdminGET(t, "/v1/admin/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var data struct {
		Email string `json:"email"`
	}
	e.DataAs(t, &data)
	if data.Email != testAdminEmail {
		t.Errorf("email = %q, quer %q", data.Email, testAdminEmail)
	}
}

// TestAdminMeDeniedNoHeaders valida 401 sem credenciais.
func TestAdminMeDeniedNoHeaders(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.GET(t, "/v1/admin/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, quer 401", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	if e.Message == "" {
		t.Error("erro sem mensagem PT-BR")
	}
}

// TestAdminMeDeniedWrongSecret valida 401 com secret errado.
func TestAdminMeDeniedWrongSecret(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	req, _ := http.NewRequest(http.MethodGet, env.BaseURL+"/v1/admin/me", nil)
	req.Header.Set("X-Pangolin-Email", testAdminEmail)
	req.Header.Set("X-Pangolin-Secret", "secret-errado")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, quer 401", resp.StatusCode)
	}
}
