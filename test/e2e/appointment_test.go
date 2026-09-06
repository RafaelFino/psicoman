package e2e

import (
	"net/http"
	"testing"
	"time"
)

// registerPortalPatient cadastra um paciente no portal e devolve o client logado.
func registerPortalPatient(t *testing.T, portalEnv *Env, email string) *PortalClient {
	t.Helper()
	pc := portalEnv.NewPortalClient(t)
	resp := pc.PUT(t, "/v1/portal/register", map[string]any{
		"credential": email, "name": "Paciente " + email, "phone": "1199",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	resp.Body.Close()
	return pc
}

func TestAppointmentFlow(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	pc := registerPortalPatient(t, portalEnv, "agenda@example.com")

	// Paciente vê a agenda aberta (vazia mas responde 200).
	resp := pc.GET(t, "/v1/portal/availability")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Paciente cria pedido de agendamento.
	start := time.Now().Add(48 * time.Hour)
	end := start.Add(time.Hour)
	resp = pc.POST(t, "/v1/portal/appointment-requests", map[string]any{
		"slot_start": start.Format(time.RFC3339),
		"slot_end":   end.Format(time.RFC3339),
		"note":       "Prefiro de manhã",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("request status = %d, quer 201", resp.StatusCode)
	}
	re := DecodeEnvelope(t, resp)
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	re.DataAs(t, &req)
	if req.Status != "pendente" {
		t.Errorf("status = %q, quer pendente", req.Status)
	}

	// Terapeuta vê o pedido na tela de pendências.
	resp = adminEnv.AdminGET(t, "/v1/admin/appointment-requests")
	pe := DecodeEnvelope(t, resp)
	var pending []map[string]any
	pe.DataAs(t, &pending)
	if len(pending) != 1 || pending[0]["id"] != req.ID {
		t.Fatalf("pendências = %v, quer 1 com id %s", pending, req.ID)
	}

	// Terapeuta confirma → cria sessão agendada + evento no Calendar.
	resp = adminEnv.AdminPOST(t, "/v1/admin/appointment-requests/"+req.ID+"/confirm", map[string]any{"modality": "online"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("confirm status = %d, quer 201", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var sess struct {
		Status  string `json:"status"`
		MeetURL string `json:"meet_url"`
	}
	ce.DataAs(t, &sess)
	if sess.Status != "agendada" {
		t.Errorf("sessão status = %q, quer agendada", sess.Status)
	}
	if sess.MeetURL == "" {
		t.Error("sessão sem link do Meet")
	}
	if len(adminEnv.Google.Events) != 1 {
		t.Errorf("eventos no Calendar = %d, quer 1", len(adminEnv.Google.Events))
	}

	// O pedido saiu das pendências.
	resp = adminEnv.AdminGET(t, "/v1/admin/appointment-requests")
	pe2 := DecodeEnvelope(t, resp)
	var pending2 []map[string]any
	pe2.DataAs(t, &pending2)
	if len(pending2) != 0 {
		t.Errorf("pendências após confirmar = %d, quer 0", len(pending2))
	}

	// Paciente vê a sessão em "minhas sessões".
	resp = pc.GET(t, "/v1/portal/sessions")
	me := DecodeEnvelope(t, resp)
	var mySessions []map[string]any
	me.DataAs(t, &mySessions)
	if len(mySessions) != 1 {
		t.Errorf("minhas sessões = %d, quer 1", len(mySessions))
	}
}

func TestConfirmBlockedByConflict(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	// Ocupa o horário desejado no Calendar (conflito).
	start := time.Now().Add(72 * time.Hour)
	end := start.Add(time.Hour)
	adminEnv.Google.SetBusy(start, end)

	pc := registerPortalPatient(t, portalEnv, "conflito2@example.com")
	resp := pc.POST(t, "/v1/portal/appointment-requests", map[string]any{
		"slot_start": start.Format(time.RFC3339), "slot_end": end.Format(time.RFC3339),
	})
	re := DecodeEnvelope(t, resp)
	var req struct {
		ID string `json:"id"`
	}
	re.DataAs(t, &req)

	// Confirmação bloqueada por conflito → 409.
	resp = adminEnv.AdminPOST(t, "/v1/admin/appointment-requests/"+req.ID+"/confirm", map[string]any{"modality": "online"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("confirm com conflito status = %d, quer 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Nenhum evento criado; pedido continua pendente.
	if len(adminEnv.Google.Events) != 0 {
		t.Errorf("eventos = %d, quer 0", len(adminEnv.Google.Events))
	}
	resp = adminEnv.AdminGET(t, "/v1/admin/appointment-requests")
	pe := DecodeEnvelope(t, resp)
	var pending []map[string]any
	pe.DataAs(t, &pending)
	if len(pending) != 1 {
		t.Errorf("pendências = %d, quer 1 (conflito não confirma)", len(pending))
	}
}
