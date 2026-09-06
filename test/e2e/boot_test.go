package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestBootAndHealth sobe o binário admin e valida os endpoints de saúde.
func TestBootAndHealth(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	for _, path := range []string{"/healthz", "/livez", "/readyz"} {
		resp := env.GET(t, path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, quer 200", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestBootPortalHealth(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	resp := env.GET(t, "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

var _ = context.Background
var _ = time.Second
