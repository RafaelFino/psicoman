package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestCostRateioPerSession(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Local com custo mensal de R$ 3.000,00 (300000 centavos).
	resp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Sala Mensal", "modality": "presencial",
		"cost_amount": 300000, "cost_period": "mensal",
	})
	le := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &loc)

	pid := env.createPatient(t, "custo@example.com")

	// Cria e finaliza 2 sessões no mesmo local/mês, com consider_cost.
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	var sessionIDs []string
	for i := 0; i < 2; i++ {
		start := base.Add(time.Duration(i) * 24 * time.Hour).Format(time.RFC3339)
		end := base.Add(time.Duration(i)*24*time.Hour + time.Hour).Format(time.RFC3339)
		resp = env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "location_id": loc.ID, "modality": "presencial",
			"starts_at": start, "ends_at": end, "status": "agendada",
		})
		se := DecodeEnvelope(t, resp)
		var s struct {
			ID string `json:"id"`
		}
		se.DataAs(t, &s)
		sessionIDs = append(sessionIDs, s.ID)

		resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{
			"bill": false, "consider_cost": true,
		})
		resp.Body.Close()
	}

	// A segunda sessão vê 2 realizadas no período → rateio = 300000/2 = 150000.
	resp = env.AdminGET(t, "/v1/admin/sessions/"+sessionIDs[1]+"/cost")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session cost status = %d", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var sc struct {
		Amount int64  `json:"amount"`
		Method string `json:"method"`
	}
	ce.DataAs(t, &sc)
	if sc.Method != "rateio" {
		t.Errorf("método = %q, quer rateio", sc.Method)
	}
	if sc.Amount != 150000 {
		t.Errorf("rateio = %d, quer 150000 (300000/2)", sc.Amount)
	}
}

func TestCostDirectPerSession(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Local com custo por_sessao de R$ 50,00.
	resp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Online", "modality": "online", "cost_amount": 5000, "cost_period": "por_sessao",
	})
	le := DecodeEnvelope(t, resp)
	var loc struct {
		ID string `json:"id"`
	}
	le.DataAs(t, &loc)

	pid := env.createPatient(t, "custodir@example.com")
	start := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(3 * time.Hour).Format(time.RFC3339)
	resp = env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "location_id": loc.ID, "modality": "online",
		"starts_at": start, "ends_at": end, "status": "agendada",
	})
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)
	env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"consider_cost": true}).Body.Close()

	resp = env.AdminGET(t, "/v1/admin/sessions/"+s.ID+"/cost")
	ce := DecodeEnvelope(t, resp)
	var sc struct {
		Amount int64  `json:"amount"`
		Method string `json:"method"`
	}
	ce.DataAs(t, &sc)
	if sc.Method != "direto" || sc.Amount != 5000 {
		t.Errorf("custo direto = {%d, %q}, quer {5000, direto}", sc.Amount, sc.Method)
	}
}

func TestROIByOrigin(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Origem Doctoralia.
	resp := env.AdminPOST(t, "/v1/admin/origins", map[string]any{"name": "Doctoralia"})
	oe := DecodeEnvelope(t, resp)
	var origin struct {
		ID string `json:"id"`
	}
	oe.DataAs(t, &origin)

	// Categoria + item de custo de plataforma ligado à origem: R$ 300/mês.
	resp = env.AdminPOST(t, "/v1/admin/costs/categories", map[string]any{"kind": "plataforma", "name": "Plataformas"})
	ce := DecodeEnvelope(t, resp)
	var cat struct {
		ID string `json:"id"`
	}
	ce.DataAs(t, &cat)
	env.AdminPOST(t, "/v1/admin/costs/items", map[string]any{
		"category_id": cat.ID, "name": "Doctoralia mensal", "amount": 30000, "period": "mensal", "origin_id": origin.ID,
	}).Body.Close()

	// Paciente dessa origem, com plano por consulta, sessão finalizada e paga.
	resp = env.AdminPOST(t, "/v1/admin/patients", map[string]any{
		"name": "Paciente ROI", "phone": "1199", "email": "roi@example.com", "origin_id": origin.ID,
	})
	pe := DecodeEnvelope(t, resp)
	var p struct {
		ID string `json:"id"`
	}
	pe.DataAs(t, &p)

	env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": p.ID, "type": "pagamento_por_consulta", "amount": 20000,
	}).Body.Close()

	start := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	end := time.Now().Add(3 * time.Hour).Format(time.RFC3339)
	resp = env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": p.ID, "modality": "online", "starts_at": start, "ends_at": end, "status": "agendada",
	})
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)
	env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"bill": true}).Body.Close()

	// Paga o débito.
	resp = env.AdminGET(t, "/v1/admin/debts?patient_id="+p.ID)
	de := DecodeEnvelope(t, resp)
	var debts []map[string]any
	de.DataAs(t, &debts)
	debtID := debts[0]["id"].(string)
	env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 20000, "method": "pix"}).Body.Close()

	// ROI do período atual: receita 20000 da origem Doctoralia.
	resp = env.AdminGET(t, "/v1/admin/reports/roi")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("roi status = %d", resp.StatusCode)
	}
	re := DecodeEnvelope(t, resp)
	var rows []map[string]any
	re.DataAs(t, &rows)

	found := false
	for _, row := range rows {
		if row["origin_id"] == origin.ID {
			found = true
			if int64(row["revenue"].(float64)) != 20000 {
				t.Errorf("receita ROI = %v, quer 20000", row["revenue"])
			}
			if int64(row["cost"].(float64)) <= 0 {
				t.Errorf("custo ROI = %v, quer > 0", row["cost"])
			}
		}
	}
	if !found {
		t.Error("origem Doctoralia ausente no relatório de ROI")
	}
}
