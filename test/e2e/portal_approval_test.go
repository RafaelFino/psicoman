package e2e

import (
	"net/http"
	"testing"
)

// TestPortalApprovalGate cobre o gate de aprovação (mvp-audit1 R1):
//   - auto-cadastro pelo portal nasce pendente;
//   - pendente é barrado (403) nas rotas de recurso, mas acessa /me e
//     /approval-status;
//   - após o terapeuta aprovar no admin, o mesmo paciente acessa os recursos.
func TestPortalApprovalGate(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	const email = "pendente@example.com"
	pc := portalEnv.NewPortalClient(t)

	// 1. Auto-cadastro pelo portal (credential = email no fake verifier).
	resp := pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": email, "name": "Paciente Pendente", "phone": "11999990000",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. /approval-status disponível e pendente.
	e := DecodeEnvelope(t, pc.GET(t, "/v1/portal/approval-status"))
	var status struct {
		Status string `json:"status"`
		Email  string `json:"email"`
	}
	e.DataAs(t, &status)
	if status.Status != "pendente" {
		t.Errorf("esperava pendente, veio %q", status.Status)
	}

	// 3. /me liberado mesmo pendente.
	if resp := pc.GET(t, "/v1/portal/me"); resp.StatusCode != http.StatusOK {
		t.Errorf("/me para pendente = %d, quer 200", resp.StatusCode)
		resp.Body.Close()
	}

	// 4. Rotas de recurso negadas (403) enquanto pendente.
	for _, path := range []string{
		"/v1/portal/availability", "/v1/portal/sessions",
		"/v1/portal/debts", "/v1/portal/appointment-requests",
	} {
		resp := pc.GET(t, path)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("GET %s pendente = %d, quer 403", path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// 5. Terapeuta encontra na fila e aprova.
	le := DecodeEnvelope(t, adminEnv.AdminGET(t, "/v1/admin/patients/pending"))
	var pendings []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	le.DataAs(t, &pendings)
	if len(pendings) != 1 || pendings[0].Email != email {
		t.Fatalf("fila de pendentes = %+v, quer 1 com %q", pendings, email)
	}
	if resp := adminEnv.AdminPOST(t, "/v1/admin/patients/"+pendings[0].ID+"/approve", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, quer 200", resp.StatusCode)
	}

	// A aprovação registra audit log (operação sensível — R1.4).
	ae := DecodeEnvelope(t, adminEnv.AdminGET(t, "/v1/admin/audit-log"))
	var entries []map[string]any
	ae.DataAs(t, &entries)
	found := false
	for _, en := range entries {
		if en["action"] == "aprovar" && en["entity"] == "patient" {
			found = true
			break
		}
	}
	if !found {
		t.Error("auditoria sem registro de aprovação (action=aprovar)")
	}

	// 6. Rotas de recurso liberadas (200) para o mesmo paciente.
	if resp := pc.GET(t, "/v1/portal/availability"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /availability após aprovação = %d, quer 200", resp.StatusCode)
		resp.Body.Close()
	}
	if resp := pc.GET(t, "/v1/portal/sessions"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /sessions após aprovação = %d, quer 200", resp.StatusCode)
		resp.Body.Close()
	}

	// 7. Status agora aprovado.
	e2 := DecodeEnvelope(t, pc.GET(t, "/v1/portal/approval-status"))
	e2.DataAs(t, &status)
	if status.Status != "aprovado" {
		t.Errorf("após aprovação esperava aprovado, veio %q", status.Status)
	}
}

// TestPortalRejectedGate: paciente rejeitado continua barrado (403).
func TestPortalRejectedGate(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	const email = "rejeitado@example.com"
	pc := portalEnv.NewPortalClient(t)
	pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": email, "name": "Rejeitado", "phone": "11888887777",
	}).Body.Close()

	le := DecodeEnvelope(t, adminEnv.AdminGET(t, "/v1/admin/patients/pending"))
	var pendings []struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &pendings)
	if len(pendings) != 1 {
		t.Fatalf("esperava 1 pendente, veio %d", len(pendings))
	}
	if resp := adminEnv.AdminPOST(t, "/v1/admin/patients/"+pendings[0].ID+"/reject", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("reject status = %d, quer 200", resp.StatusCode)
	}

	resp := pc.GET(t, "/v1/portal/availability")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /availability rejeitado = %d, quer 403", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestAdminCreatedPatientIsApproved: paciente cadastrado pelo terapeuta já
// nasce aprovado (não aparece na fila de pendentes) — R1.1.
func TestAdminCreatedPatientIsApproved(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Direto Admin", "phone": "1199", "email": "direto@example.com",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var created struct {
		ApprovalStatus string `json:"approval_status"`
	}
	ce.DataAs(t, &created)
	if created.ApprovalStatus != "aprovado" {
		t.Errorf("cadastro admin nasce %q, quer aprovado", created.ApprovalStatus)
	}

	le := DecodeEnvelope(t, env.AdminGET(t, "/v1/admin/patients/pending"))
	var pendings []map[string]any
	le.DataAs(t, &pendings)
	for _, p := range pendings {
		if p["email"] == "direto@example.com" {
			t.Errorf("paciente do admin não deveria estar na fila de pendentes")
		}
	}
}
