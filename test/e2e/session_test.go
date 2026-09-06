package e2e

import (
	"net/http"
	"testing"
	"time"
)

// createPatient cria um paciente e devolve o id (helper para outros testes).
func (e *Env) createPatient(t *testing.T, email string) string {
	t.Helper()
	resp := e.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Paciente " + email, "phone": "1199", "email": email,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar paciente status = %d", resp.StatusCode)
	}
	env := DecodeEnvelope(t, resp)
	var p struct {
		ID string `json:"id"`
	}
	env.DataAs(t, &p)
	return p.ID
}

func TestSessionLifecycle(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "sessao@example.com")
	start := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(25 * time.Hour).Format(time.RFC3339)

	// Cria como "solicitada".
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "solicitada",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, quer 201", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var s struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	ce.DataAs(t, &s)
	if s.Status != "solicitada" {
		t.Errorf("status inicial = %q, quer solicitada", s.Status)
	}

	// Agenda.
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/schedule", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schedule status = %d, quer 200", resp.StatusCode)
	}
	se := DecodeEnvelope(t, resp)
	var sched struct {
		Status string `json:"status"`
	}
	se.DataAs(t, &sched)
	if sched.Status != "agendada" {
		t.Errorf("status = %q, quer agendada", sched.Status)
	}

	// Finaliza com cobrança e custo.
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{
		"bill": true, "consider_cost": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finish status = %d, quer 200", resp.StatusCode)
	}
	fe := DecodeEnvelope(t, resp)
	var fin struct {
		Status       string `json:"status"`
		Bill         bool   `json:"bill"`
		ConsiderCost bool   `json:"consider_cost"`
	}
	fe.DataAs(t, &fin)
	if fin.Status != "realizada" || !fin.Bill || !fin.ConsiderCost {
		t.Errorf("finalização inconsistente: %+v", fin)
	}

	// Finalizar de novo → 422 (já terminal).
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"bill": false})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("re-finish status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSessionCancelAndNoShow(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "cancel@example.com")
	start := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(49 * time.Hour).Format(time.RFC3339)

	mk := func() string {
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "presencial",
			"starts_at": start, "ends_at": end, "status": "agendada",
		})
		e := DecodeEnvelope(t, resp)
		var s struct {
			ID string `json:"id"`
		}
		e.DataAs(t, &s)
		return s.ID
	}

	// Cancelar a partir de agendada.
	id1 := mk()
	resp := env.AdminPOST(t, "/v1/admin/sessions/"+id1+"/cancel", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("cancel status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Falta a partir de agendada.
	id2 := mk()
	resp = env.AdminPOST(t, "/v1/admin/sessions/"+id2+"/no-show", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("no-show status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSessionInvalidTransition(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "invalid@example.com")
	start := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(73 * time.Hour).Format(time.RFC3339)

	// Cria como solicitada e tenta finalizar direto → 422.
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "solicitada",
	})
	e := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	e.DataAs(t, &s)

	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"bill": true})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("finish de solicitada status = %d, quer 422", resp.StatusCode)
	}
	resp.Body.Close()
}
