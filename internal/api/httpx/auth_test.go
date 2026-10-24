package httpx

import (
	"net/http"
	"testing"
)

// TestPangolinHeaderNameDefault: sem env, PangolinHeaderName cai no default.
func TestPangolinHeaderNameDefault(t *testing.T) {
	t.Setenv(EnvPangolinIdentityHeader, "")
	if got := PangolinHeaderName(); got != DefaultPangolinIdentityHeader {
		t.Errorf("default esperado %q, veio %q", DefaultPangolinIdentityHeader, got)
	}
}

// TestPangolinHeaderNameEnvOverride: env define o nome do header.
func TestPangolinHeaderNameEnvOverride(t *testing.T) {
	t.Setenv(EnvPangolinIdentityHeader, "  X-Custom-Id  ")
	if got := PangolinHeaderName(); got != "X-Custom-Id" {
		t.Errorf("override esperado X-Custom-Id, veio %q", got)
	}
}

// TestRequirePangolinSemHeader401: header ausente/vazio → 401 nao_autenticado.
func TestRequirePangolinSemHeader401(t *testing.T) {
	e, rec := newEvent(t)
	mw := RequirePangolinIdentity(DefaultPangolinIdentityHeader)
	if err := mw(e); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("quero 401, veio %d", rec.Code)
	}
	if body := rec.Body.String(); !contains(body, `"code":"nao_autenticado"`) {
		t.Errorf("code esperado nao_autenticado: %s", body)
	}
}

// TestRequirePangolinComHeaderPassaEExpoeIdentidade: header presente → passa
// (sem resposta de erro) e a identidade fica disponível via ActorIdentity
// (preparação de auditoria).
func TestRequirePangolinComHeaderPassaEExpoeIdentidade(t *testing.T) {
	e, rec := newEvent(t)
	e.Request.Header.Set(DefaultPangolinIdentityHeader, "  terapeuta@exemplo.com  ")
	mw := RequirePangolinIdentity(DefaultPangolinIdentityHeader)
	if err := mw(e); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	// e.Next() é no-op (sem próximo handler); não deve ter escrito erro.
	if rec.Code != http.StatusOK {
		t.Fatalf("header válido não deveria responder erro, veio %d (%s)", rec.Code, rec.Body.String())
	}
	if got := ActorIdentity(e); got != "terapeuta@exemplo.com" {
		t.Errorf("ActorIdentity esperado terapeuta@exemplo.com (trim), veio %q", got)
	}
}

// TestRequirePangolinNomeConfiguravel: o middleware construído com um nome
// custom aceita o header custom e NÃO o default.
func TestRequirePangolinNomeConfiguravel(t *testing.T) {
	mw := RequirePangolinIdentity("X-Custom-Id")

	// Só o header custom é aceito.
	e1, rec1 := newEvent(t)
	e1.Request.Header.Set("X-Custom-Id", "op@exemplo.com")
	if err := mw(e1); err != nil {
		t.Fatalf("middleware (custom): %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Errorf("header custom deveria passar, veio %d", rec1.Code)
	}

	// O default NÃO é aceito quando o nome configurado é outro.
	e2, rec2 := newEvent(t)
	e2.Request.Header.Set(DefaultPangolinIdentityHeader, "op@exemplo.com")
	if err := mw(e2); err != nil {
		t.Fatalf("middleware (default): %v", err)
	}
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("default não deveria ser aceito com nome custom, veio %d", rec2.Code)
	}
}

// TestPortalPatientIDRoundTrip: SetPortalPatientID/PortalPatientID round-trip.
func TestPortalPatientIDRoundTrip(t *testing.T) {
	e, _ := newEvent(t)
	if got := PortalPatientID(e); got != "" {
		t.Errorf("sem set, esperava \"\", veio %q", got)
	}
	SetPortalPatientID(e, "01ABC")
	if got := PortalPatientID(e); got != "01ABC" {
		t.Errorf("esperava 01ABC, veio %q", got)
	}
}

// TestPortalAuthRecordIDSemSessao: sem sessão (e.Auth nil), PortalAuthRecordID
// devolve "". O caminho com sessão real (auth record patient_user) é coberto
// pelos testes de gate em processo no pacote api.
func TestPortalAuthRecordIDSemSessao(t *testing.T) {
	e, _ := newEvent(t)
	if got := PortalAuthRecordID(e); got != "" {
		t.Errorf("sem sessão, esperava \"\", veio %q", got)
	}
}

// contains é um helper local para evitar import de strings só por Contains.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
