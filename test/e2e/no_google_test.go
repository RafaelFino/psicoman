package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestAgendarSemGoogle garante que, sem Google configurado (dev local /
// instância não autorizada), criar uma sessão JÁ COMO "agendada" funciona
// (201) em vez de estourar 500 ao tentar criar o evento no Calendar.
// Regressão: antes, o cliente Google real era injetado mesmo sem credenciais e
// a criação do evento falhava no OAuth → 500.
func TestAgendarSemGoogle(t *testing.T) {
	env := StartAdminNoGoogle(t)
	defer env.Stop()

	pid := env.createPatient(t, "sem-google@example.com")
	start := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(25 * time.Hour).Format(time.RFC3339)

	// Agendada direto: dispara o caminho de criação de evento (no-op sem Google).
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "agendada",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("agendar sem Google status = %d, quer 201", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var s struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		MeetURL string `json:"meet_url"`
	}
	e.DataAs(t, &s)
	if s.Status != "agendada" {
		t.Errorf("status = %q, quer agendada", s.Status)
	}
	// Sem Google, não há link do Meet — e isso não é erro.
	if s.MeetURL != "" {
		t.Errorf("meet_url deveria ser vazio sem Google, veio %q", s.MeetURL)
	}

	// Finalizar também deve funcionar normalmente.
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"consider_cost": false})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("finish sem Google status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestBackupSemGoogle garante que o backup/restore sem Drive configurado
// devolve erro tratável (422), não um 500 por nil panic.
func TestBackupSemGoogle(t *testing.T) {
	env := StartAdminNoGoogle(t)
	defer env.Stop()

	resp := env.AdminPOST(t, "/v1/admin/backup", map[string]any{})
	// Esperado: validação (indisponível), não 500.
	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("backup sem Drive não deveria dar 500 (nil panic); veio %d", resp.StatusCode)
	}
	resp.Body.Close()
}
