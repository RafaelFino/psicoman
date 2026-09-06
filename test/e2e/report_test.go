package e2e

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestFinancialReport(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Gera um débito por sessão (R$ 200) e paga metade.
	pid := env.finishBilledSession(t, "pagamento_por_consulta", 20000)
	resp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	de := DecodeEnvelope(t, resp)
	var debts []map[string]any
	de.DataAs(t, &debts)
	debtID := debts[0]["id"].(string)
	env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 10000}).Body.Close()

	// Relatório financeiro do dia (janela ampla cobrindo agora).
	from := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	q := url.Values{"from": {from}, "to": {to}}

	resp = env.AdminGET(t, "/v1/admin/reports/financial?"+q.Encode())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("financial status = %d", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var fin struct {
		Generated int64 `json:"generated"`
		Open      int64 `json:"open"`
		Received  int64 `json:"received"`
	}
	e.DataAs(t, &fin)
	if fin.Generated != 20000 {
		t.Errorf("gerado = %d, quer 20000", fin.Generated)
	}
	if fin.Received != 10000 {
		t.Errorf("recebido = %d, quer 10000", fin.Received)
	}
	// Débito ficou parcial → ainda em aberto (valor total do débito parcial).
	if fin.Open != 20000 {
		t.Errorf("em aberto = %d, quer 20000 (parcial conta como aberto)", fin.Open)
	}
}

func TestCostReportByKindAndPatient(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Custo de infra (Google) mensal R$ 100.
	resp := env.AdminPOST(t, "/v1/admin/costs/categories", map[string]any{"kind": "infra", "name": "Infra"})
	ce := DecodeEnvelope(t, resp)
	var cat struct {
		ID string `json:"id"`
	}
	ce.DataAs(t, &cat)
	env.AdminPOST(t, "/v1/admin/costs/items", map[string]any{
		"category_id": cat.ID, "name": "Google", "amount": 10000, "period": "mensal",
	}).Body.Close()

	// Local por_sessao + sessão com consider_cost para custo por paciente.
	resp = env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Sala", "modality": "presencial", "cost_amount": 8000, "cost_period": "por_sessao",
	})
	le := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &loc)

	pid := env.createPatient(t, "custorel@example.com")
	base := time.Now().UTC()
	start := base.Add(2 * time.Hour).Format(time.RFC3339)
	end := base.Add(3 * time.Hour).Format(time.RFC3339)
	resp = env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "location_id": loc.ID, "modality": "presencial",
		"starts_at": start, "ends_at": end, "status": "agendada",
	})
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)
	env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"consider_cost": true}).Body.Close()

	from := base.Add(-24 * time.Hour).Format(time.RFC3339)
	to := base.Add(24 * time.Hour).Format(time.RFC3339)
	q := url.Values{"from": {from}, "to": {to}}

	resp = env.AdminGET(t, "/v1/admin/reports/costs?"+q.Encode())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("costs status = %d", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var rep struct {
		ByKind    map[string]int64 `json:"by_kind"`
		ByPatient map[string]int64 `json:"by_patient"`
	}
	e.DataAs(t, &rep)

	if rep.ByKind["infra"] <= 0 {
		t.Errorf("custo infra = %d, quer > 0", rep.ByKind["infra"])
	}
	if rep.ByPatient[pid] != 8000 {
		t.Errorf("custo do paciente = %d, quer 8000", rep.ByPatient[pid])
	}
}
