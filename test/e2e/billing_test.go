package e2e

import (
	"net/http"
	"testing"
	"time"
)

// finishBilledSession cria paciente+plano, cria e finaliza uma sessão faturável.
func (e *Env) finishBilledSession(t *testing.T, planType string, amount int64) (patientID string) {
	t.Helper()
	patientID = e.createPatient(t, planType+"@example.com")

	// Plano.
	resp := e.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": patientID, "type": planType, "amount": amount,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar plano status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Sessão agendada.
	start := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(3 * time.Hour).Format(time.RFC3339)
	resp = e.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": patientID, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "agendada",
	})
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)

	// Finaliza faturável.
	resp = e.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{
		"bill": true, "consider_cost": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("finish status = %d", resp.StatusCode)
	}
	resp.Body.Close()
	return patientID
}

func TestBillingPerConsultaGeneratesDebt(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.finishBilledSession(t, "pagamento_por_consulta", 15000)

	resp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	e := DecodeEnvelope(t, resp)
	var debts []map[string]any
	e.DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	if int64(debts[0]["amount"].(float64)) != 15000 {
		t.Errorf("valor do débito = %v, quer 15000", debts[0]["amount"])
	}
}

func TestBillingSocialNoDebtE2E(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.finishBilledSession(t, "atendimento_social", 0)

	resp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	e := DecodeEnvelope(t, resp)
	var debts []map[string]any
	e.DataAs(t, &debts)
	if len(debts) != 0 {
		t.Errorf("débitos = %d, quer 0 (social)", len(debts))
	}
}

func TestBillingFixedCycleE2E(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "fechado@example.com")
	// Plano fechado mensal vigente desde ontem.
	start := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	resp := env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": pid, "type": "plano_fechado_mensal", "amount": 50000, "starts_at": start,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("plano status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fecha ciclo: gera 1 débito.
	resp = env.AdminPOST(t, "/v1/admin/billing/close-cycles", map[string]any{})
	ce := DecodeEnvelope(t, resp)
	var res struct {
		DebtsCreated int `json:"debts_created"`
	}
	ce.DataAs(t, &res)
	if res.DebtsCreated < 1 {
		t.Errorf("debts_created = %d, quer >=1", res.DebtsCreated)
	}

	// Reexecutar não duplica.
	resp = env.AdminPOST(t, "/v1/admin/billing/close-cycles", map[string]any{})
	ce2 := DecodeEnvelope(t, resp)
	var res2 struct {
		DebtsCreated int `json:"debts_created"`
	}
	ce2.DataAs(t, &res2)
	if res2.DebtsCreated != 0 {
		t.Errorf("reexecução debts_created = %d, quer 0", res2.DebtsCreated)
	}

	// Paciente tem exatamente 1 débito de ciclo (session_id vazio).
	resp = env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	de := DecodeEnvelope(t, resp)
	var debts []map[string]any
	de.DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	if debts[0]["billing_period"] == "" {
		t.Error("débito de ciclo deveria ter billing_period")
	}
}
