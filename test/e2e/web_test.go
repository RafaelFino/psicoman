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
	// R3.2: telefone vira link de WhatsApp (helper Wa.link no markup).
	if !strings.Contains(string(appBody), "Wa.link(") {
		t.Error("markup do admin sem link de WhatsApp (Wa.link)")
	}
	// R1: aba Prontuário presente (substitui Sessões/Notas).
	if !strings.Contains(string(appBody), "tab === 'prontuario'") {
		t.Error("markup do admin sem a aba Prontuário")
	}
	if strings.Contains(string(appBody), "tab === 'sessoes'") || strings.Contains(string(appBody), "tab === 'notas'") {
		t.Error("abas antigas (Sessões/Notas) ainda presentes no markup")
	}

	// CSS estático.
	resp = env.GET(t, "/static/app.css")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("css status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// R5.1: slot da agenda abre o painel da sessão (não só o paciente).
	if !strings.Contains(string(appBody), "openSession(s)") {
		t.Error("slot da agenda não abre o painel da sessão")
	}

	// R5.1: CSS da semana fluida (colunas alinhadas pelo topo).
	resp = env.GET(t, "/static/app.css")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("app.css status = %d", resp.StatusCode)
	}
	cssBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(cssBody), "align-items: start") {
		t.Error("week-grid sem align-items: start (dia que cresce com a lotação)")
	}

	// JS estático: helper Wa disponível no cliente.
	resp = env.GET(t, "/static/api.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("api.js status = %d, quer 200", resp.StatusCode)
	}
	jsBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(jsBody), "const Wa") {
		t.Error("api.js sem helper Wa")
	}
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
