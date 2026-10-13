package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestDayFeed popula eventos de hoje e confere o feed do dia nas três janelas.
func TestDayFeed(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "dia@example.com")
	env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": pid, "type": "pagamento_por_consulta", "amount": 15000,
	}).Body.Close()

	// Sessão de hoje (daqui a 2h), finalizada → gera débito.
	start := time.Now().Add(2 * time.Hour)
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start.Format(time.RFC3339), "ends_at": start.Add(time.Hour).Format(time.RFC3339),
		"status": "agendada",
	})
	var s struct {
		ID string `json:"id"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &s)
	env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/finish", map[string]any{"consider_cost": false}).Body.Close()

	// Débito gerado → pagar (pagamento de hoje).
	dresp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	var debts []map[string]any
	DecodeEnvelope(t, dresp).DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	debtID := debts[0]["id"].(string)
	env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 15000, "method": "pix"}).Body.Close()

	// Arquivo anexado hoje.
	up := env.uploadFile(t, "/v1/admin/patients/"+pid+"/files", "doc.txt", []byte("conteúdo"))
	up.Body.Close()

	// Feed do dia (hoje).
	resp = env.AdminGET(t, "/v1/admin/day-feed?window=hoje")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("day-feed status = %d", resp.StatusCode)
	}
	var feed struct {
		Window string `json:"window"`
		Resumo struct {
			Total    int   `json:"total"`
			Recebido int64 `json:"recebido"`
		} `json:"resumo"`
		Eventos []struct {
			Tipo        string `json:"tipo"`
			PatientName string `json:"patient_name"`
		} `json:"eventos"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &feed)

	if feed.Window != "hoje" {
		t.Errorf("window = %q, quer hoje", feed.Window)
	}
	tipos := map[string]int{}
	for _, e := range feed.Eventos {
		tipos[e.Tipo]++
	}
	if tipos["sessao"] < 1 {
		t.Errorf("feed sem sessão de hoje: %+v", tipos)
	}
	if tipos["debito"] < 1 {
		t.Errorf("feed sem débito de hoje: %+v", tipos)
	}
	if tipos["pagamento"] < 1 {
		t.Errorf("feed sem pagamento de hoje: %+v", tipos)
	}
	if tipos["arquivo"] < 1 {
		t.Errorf("feed sem arquivo de hoje: %+v", tipos)
	}
	if feed.Resumo.Recebido != 15000 {
		t.Errorf("recebido = %d, quer 15000", feed.Resumo.Recebido)
	}

	// Janela "recentes": também deve conter os eventos de hoje.
	resp = env.AdminGET(t, "/v1/admin/day-feed?window=recentes")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("day-feed recentes status = %d", resp.StatusCode)
	}
	var rec struct {
		Window  string `json:"window"`
		Eventos []any  `json:"eventos"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &rec)
	if rec.Window != "recentes" {
		t.Errorf("window recentes = %q", rec.Window)
	}

	// Janela inválida cai em "hoje".
	resp = env.AdminGET(t, "/v1/admin/day-feed?window=xpto")
	var inv struct {
		Window string `json:"window"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &inv)
	if inv.Window != "hoje" {
		t.Errorf("janela inválida deveria cair em hoje, veio %q", inv.Window)
	}
}
