package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAdminWebPages(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Home.
	resp := env.GET(t, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("home status = %d, quer 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "administração") {
		t.Error("home admin sem conteúdo esperado")
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}

	// App (painel).
	resp = env.GET(t, "/app/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("app status = %d, quer 200", resp.StatusCode)
	}
	appBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	// WCAG: campos com <label> associado.
	if !strings.Contains(string(appBody), `for="p-email"`) {
		t.Error("formulário sem label associado")
	}

	// CSS estático.
	resp = env.GET(t, "/static/app.css")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("css status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPortalWebPages(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	resp := env.GET(t, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("home portal status = %d, quer 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Entrar com Google") {
		t.Error("home portal sem botão de login")
	}

	resp = env.GET(t, "/app/")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("app portal status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}
