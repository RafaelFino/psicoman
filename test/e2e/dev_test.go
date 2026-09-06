package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
)

// StartAdminDev sobe o admin em modo dev (autenticação desligada).
func StartAdminDev(t *testing.T) *Env {
	t.Helper()
	cfg := testConfig(t)
	cfg.Dev = true
	fake := google.NewFakeClient()
	env := start(t, cfg, cfg.Admin.Addr(), func(ctx context.Context, c *config.Config) (app.Runnable, error) {
		return app.NewAdminForTest(ctx, c, app.Options{Calendar: fake, Gmail: fake, Drive: fake})
	})
	env.Google = fake
	return env
}

// TestDevModeAdminNoAuth valida que, em dev, o admin responde sem headers.
func TestDevModeAdminNoAuth(t *testing.T) {
	env := StartAdminDev(t)
	defer env.Stop()

	// GET /me SEM headers de autenticação → 200 (auth desligada).
	resp := env.GET(t, "/v1/admin/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dev /me status = %d, quer 200 (auth off)", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var data struct {
		Email string `json:"email"`
	}
	e.DataAs(t, &data)
	if data.Email == "" {
		t.Error("dev mode não injetou ator")
	}

	// Operação real sem auth: cadastrar paciente.
	resp = env.GET(t, "/v1/admin/patients")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("listar pacientes sem auth (dev) = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}
