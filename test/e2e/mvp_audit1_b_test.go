package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestLocationsAndAvailability cobre B2: CRUD de local + disponibilidade, e
// verifica que a agenda aberta do portal (paciente aprovado) lista as janelas.
func TestLocationsAndAvailability(t *testing.T) {
	adminEnv, portalEnv := StartAdminAndPortal(t)
	defer adminEnv.Stop()
	defer portalEnv.Stop()

	// Cria um local com custo em centavos.
	ce := DecodeEnvelope(t, adminEnv.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Consultório Centro", "address": "Rua X, 100",
		"modality": "presencial", "cost_amount": 25000, "cost_period": "mensal",
	}))
	var loc struct {
		ID         string `json:"id"`
		CostAmount int64  `json:"cost_amount"`
	}
	ce.DataAs(t, &loc)
	if loc.ID == "" || loc.CostAmount != 25000 {
		t.Fatalf("local criado inconsistente: %+v", loc)
	}

	// Adiciona uma janela de disponibilidade (segunda 08:00-12:00).
	if resp := adminEnv.AdminPOST(t, "/v1/admin/locations/"+loc.ID+"/availability", map[string]any{
		"weekday": 1, "start_time": "08:00", "end_time": "12:00", "capacity": 1,
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("add availability status = %d, quer 201", resp.StatusCode)
	}

	// A janela aparece na listagem do admin.
	le := DecodeEnvelope(t, adminEnv.AdminGET(t, "/v1/admin/locations/"+loc.ID+"/availability"))
	var windows []map[string]any
	le.DataAs(t, &windows)
	if len(windows) != 1 {
		t.Fatalf("janelas = %d, quer 1", len(windows))
	}

	// Paciente aprovado vê a janela na agenda aberta do portal.
	pc := registerPortalPatient(t, adminEnv, portalEnv, "veagenda@example.com")
	ae := DecodeEnvelope(t, pc.GET(t, "/v1/portal/availability"))
	var avail []struct {
		LocationID string           `json:"location_id"`
		Slots      []map[string]any `json:"slots"`
	}
	ae.DataAs(t, &avail)
	found := false
	for _, a := range avail {
		if a.LocationID == loc.ID && len(a.Slots) == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("portal não listou a janela do local; veio %+v", avail)
	}
}

// TestOriginAndPatientOrigin cobre B3: origem + campo origin_id no paciente.
func TestOriginAndPatientOrigin(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	oe := DecodeEnvelope(t, env.AdminPOST(t, "/v1/admin/origins", map[string]any{"name": "Doctoralia"}))
	var origin struct {
		ID string `json:"id"`
	}
	oe.DataAs(t, &origin)
	if origin.ID == "" {
		t.Fatal("origem sem id")
	}

	pe := DecodeEnvelope(t, env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Com Origem", "phone": "1199", "email": "origem@example.com", "origin_id": origin.ID,
	}))
	var p struct {
		ID       string `json:"id"`
		OriginID string `json:"origin_id"`
	}
	pe.DataAs(t, &p)
	if p.OriginID != origin.ID {
		t.Errorf("origin_id do paciente = %q, quer %q", p.OriginID, origin.ID)
	}
}

// TestPlansByType cobre B4: criação de planos por tipo e remoção.
func TestPlansByType(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pe := DecodeEnvelope(t, env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Planos", "phone": "1199", "email": "planos@example.com",
	}))
	var p struct {
		ID string `json:"id"`
	}
	pe.DataAs(t, &p)

	now := time.Now().Format(time.RFC3339)
	cases := []struct {
		typ    string
		amount int64
	}{
		{"pagamento_por_consulta", 0},
		{"plano_fechado_mensal", 30000},
		{"atendimento_social", 0},
	}
	var lastPlanID string
	for _, c := range cases {
		body := map[string]any{"patient_id": p.ID, "type": c.typ, "amount": c.amount, "starts_at": now}
		resp := env.AdminPOST(t, "/v1/admin/plans", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("plano %s status = %d, quer 201", c.typ, resp.StatusCode)
		}
		var pl struct {
			ID string `json:"id"`
		}
		DecodeEnvelope(t, resp).DataAs(t, &pl)
		lastPlanID = pl.ID
	}

	// Lista traz os três planos.
	le := DecodeEnvelope(t, env.AdminGET(t, "/v1/admin/plans?patient_id="+p.ID))
	var plans []map[string]any
	le.DataAs(t, &plans)
	if len(plans) != 3 {
		t.Fatalf("planos = %d, quer 3", len(plans))
	}

	// Remove um.
	if resp := env.AdminDELETE(t, "/v1/admin/plans/"+lastPlanID); resp.StatusCode != http.StatusOK {
		t.Fatalf("remover plano status = %d, quer 200", resp.StatusCode)
	}
}

// TestCreateSessionFromAgenda cobre B5: agendar sessão direto (status agendada)
// e a checagem de conflito (409).
func TestCreateSessionFromAgenda(t *testing.T) {
	adminEnv := StartAdmin(t)
	defer adminEnv.Stop()

	pe := DecodeEnvelope(t, adminEnv.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Agendar", "phone": "1199", "email": "agendar@example.com",
	}))
	var p struct {
		ID string `json:"id"`
	}
	pe.DataAs(t, &p)

	start := time.Now().Add(72 * time.Hour)
	end := start.Add(time.Hour)

	// Agenda uma sessão (sem conflito) → agendada + evento no Calendar.
	resp := adminEnv.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": p.ID, "modality": "online", "status": "agendada",
		"starts_at": start.Format(time.RFC3339), "ends_at": end.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("agendar sessão status = %d, quer 201", resp.StatusCode)
	}
	var sess struct {
		Status string `json:"status"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &sess)
	if sess.Status != "agendada" {
		t.Errorf("status = %q, quer agendada", sess.Status)
	}

	// Segundo agendamento no mesmo horário → conflito (o próprio evento criado
	// marca o freebusy do fake) → 409.
	conflictStart := time.Now().Add(96 * time.Hour)
	conflictEnd := conflictStart.Add(time.Hour)
	adminEnv.Google.SetBusy(conflictStart, conflictEnd)
	resp = adminEnv.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": p.ID, "modality": "online", "status": "agendada",
		"starts_at": conflictStart.Format(time.RFC3339), "ends_at": conflictEnd.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("agendar com conflito status = %d, quer 409", resp.StatusCode)
	}
}
