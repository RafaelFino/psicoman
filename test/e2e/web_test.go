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
	// patient-timeline R5: título da aba sempre com a marca "Psicoman".
	if !strings.Contains(string(body), "<title>Psicoman · ") {
		t.Error("título da página admin sem a marca Psicoman")
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

	// patient-timeline R4: timeline é a visão base do paciente.
	if !strings.Contains(string(appBody), "tab === 'timeline'") {
		t.Error("markup do admin sem a aba Timeline")
	}
	if !strings.Contains(string(appBody), "historyFiltered()") {
		t.Error("markup da timeline sem a lista filtrável de eventos")
	}

	// patient-timeline R2: calendário mensal do paciente.
	if !strings.Contains(string(appBody), "tab === 'calendario'") {
		t.Error("markup do admin sem a aba Calendário do paciente")
	}
	if !strings.Contains(string(appBody), "loadCalendar()") {
		t.Error("markup do calendário sem a carga de sessões do mês")
	}

	// patient-timeline R6.4/R7: métricas do paciente e relatório por local.
	if !strings.Contains(string(appBody), "session-metrics") {
		t.Error("markup sem o bloco de métricas de sessão do paciente")
	}
	if !strings.Contains(string(appBody), `id="relatorio-local"`) {
		t.Error("markup sem a seção de relatório por local")
	}

	// patient-timeline R8: feed do dia é a entrada do app (seção #hoje) com resumo colapsável.
	if !strings.Contains(string(appBody), `id="hoje"`) {
		t.Error("markup sem a seção Hoje (feed do dia)")
	}
	if !strings.Contains(string(appBody), "dayFeedPanel()") {
		t.Error("markup sem o componente do feed do dia")
	}
	if !strings.Contains(string(appBody), `aria-controls="resumo-dia"`) {
		t.Error("feed do dia sem o resumo colapsável acessível")
	}

	// patient-timeline R3: prontuário com composição rápida e anamnese como ficha inicial.
	if !strings.Contains(string(appBody), "Registrar o que aconteceu") {
		t.Error("prontuário sem a composição rápida de nota no topo")
	}
	if !strings.Contains(string(appBody), "Ficha inicial") {
		t.Error("prontuário sem a anamnese destacada como ficha inicial")
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
	// patient-timeline D1: helper de data relativa para a timeline.
	if !strings.Contains(string(jsBody), "relative(iso)") {
		t.Error("api.js sem helper Fmt.relative")
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
	// patient-timeline R5: título do portal também com a marca.
	if !strings.Contains(string(body), "<title>Psicoman · ") {
		t.Error("título da página do portal sem a marca Psicoman")
	}

	resp = env.GET(t, "/app/")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("app portal status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}
