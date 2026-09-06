package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestAuditLogRecorded(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Uma operação sensível: cadastrar paciente (gera audit "criar").
	env.createPatient(t, "audit@example.com")

	resp := env.AdminGET(t, "/v1/admin/audit-log")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit-log status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var entries []map[string]any
	e.DataAs(t, &entries)
	if len(entries) == 0 {
		t.Fatal("nenhuma entrada de auditoria registrada")
	}
	// Deve haver ao menos um login_sucesso (auth) e um criar (paciente).
	actions := map[string]bool{}
	for _, en := range entries {
		if a, ok := en["action"].(string); ok {
			actions[a] = true
		}
	}
	if !actions["login_sucesso"] {
		t.Error("auditoria sem login_sucesso")
	}
	if !actions["criar"] {
		t.Error("auditoria sem registro de criação de paciente")
	}
}

func TestSwaggerSpecValid(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.GET(t, "/v1/swagger.json")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("swagger.json status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("spec não é JSON válido: %v", err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok || len(paths) < 10 {
		t.Errorf("spec com poucos paths: %d", len(paths))
	}
	// Rotas-chave documentadas.
	for _, p := range []string{"/v1/admin/patients", "/v1/portal/me", "/v1/admin/backup"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("rota %q ausente na spec", p)
		}
	}
}
