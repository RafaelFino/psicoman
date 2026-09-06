package e2e

import (
	"net/http"
	"testing"
)

func TestPortalRegisterAndMe(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	pc := env.NewPortalClient(t)
	// Cadastro básico (login social fake: credential = email).
	resp := pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": "paciente@example.com",
		"name":       "Paciente Portal",
		"phone":      "11999998888",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// GET /me usa a sessão emitida no cadastro.
	resp = pc.GET(t, "/v1/portal/me")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var me struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	e.DataAs(t, &me)
	if me.Email != "paciente@example.com" {
		t.Errorf("email = %q", me.Email)
	}
	if me.Name != "Paciente Portal" {
		t.Errorf("nome = %q", me.Name)
	}
}

func TestPortalRequiresSession(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	// Sem login, /me deve dar 401.
	pc := env.NewPortalClient(t)
	resp := pc.GET(t, "/v1/portal/me")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("me sem sessão status = %d, quer 401", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPortalIsolation(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	// Paciente A cadastra e loga.
	a := env.NewPortalClient(t)
	a.PUT(t, "/v1/portal/register", map[string]any{
		"credential": "a@example.com", "name": "A", "phone": "111",
	}).Body.Close()

	// Paciente B cadastra e loga (jar separado).
	b := env.NewPortalClient(t)
	b.PUT(t, "/v1/portal/register", map[string]any{
		"credential": "b@example.com", "name": "B", "phone": "222",
	}).Body.Close()

	// Cada um vê apenas o próprio perfil.
	resp := a.GET(t, "/v1/portal/me")
	ea := DecodeEnvelope(t, resp)
	var ma struct {
		Email string `json:"email"`
	}
	ea.DataAs(t, &ma)
	if ma.Email != "a@example.com" {
		t.Errorf("A vê email %q, quer a@example.com", ma.Email)
	}

	resp = b.GET(t, "/v1/portal/me")
	eb := DecodeEnvelope(t, resp)
	var mb struct {
		Email string `json:"email"`
	}
	eb.DataAs(t, &mb)
	if mb.Email != "b@example.com" {
		t.Errorf("B vê email %q, quer b@example.com", mb.Email)
	}
}

func TestPortalLinkByEmail(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	// Terapeuta cadastra o paciente pelo admin.
	resp := adminEnv.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Cadastrado pelo Admin", "phone": "1199", "email": "vinculo@example.com",
	})
	ae := DecodeEnvelope(t, resp)
	var created struct {
		ID string `json:"id"`
	}
	ae.DataAs(t, &created)

	// Paciente se cadastra no portal com o MESMO email → vincula, não duplica.
	pc := portalEnv.NewPortalClient(t)
	resp = pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": "vinculo@example.com", "name": "Nome do Portal", "phone": "1188",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register portal status = %d", resp.StatusCode)
	}
	pe := DecodeEnvelope(t, resp)
	var linked struct {
		ID string `json:"id"`
	}
	pe.DataAs(t, &linked)
	if linked.ID != created.ID {
		t.Errorf("vínculo falhou: id portal %q != admin %q (duplicou)", linked.ID, created.ID)
	}

	// No admin, continua havendo apenas 1 paciente com esse email.
	resp = adminEnv.AdminGET(t, "/v1/admin/patients")
	le := DecodeEnvelope(t, resp)
	var list []map[string]any
	le.DataAs(t, &list)
	count := 0
	for _, p := range list {
		if p["email"] == "vinculo@example.com" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("pacientes com o email = %d, quer 1 (sem duplicação)", count)
	}
}

func TestPortalRateLimit(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()
	// Config de teste usa limites altos (1000). Aqui validamos que a rota
	// pública responde normalmente sob uso legítimo; o limite em si é testado
	// via unit do RateLimiter.
	pc := env.NewPortalClient(t)
	resp := pc.POST(t, "/v1/portal/login", map[string]any{"credential": "rl@example.com"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}
