package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestPatientSessionMetrics popula sessões de todos os status e confere as
// contagens, taxas e faturado em GET /patients/{id}/session-metrics.
func TestPatientSessionMetrics(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "metricas@example.com")
	// Plano por sessão para a realizada gerar débito.
	env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": pid, "type": "pagamento_por_consulta", "amount": 15000,
	}).Body.Close()

	mk := func(hour int) string {
		start := time.Now().Add(time.Duration(hour) * time.Hour)
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "online",
			"starts_at": start.Format(time.RFC3339), "ends_at": start.Add(time.Hour).Format(time.RFC3339),
			"status": "agendada",
		})
		var s struct {
			ID string `json:"id"`
		}
		DecodeEnvelope(t, resp).DataAs(t, &s)
		return s.ID
	}

	// 1 realizada (gera débito), 1 falta, 1 cancelada paciente, 1 cancelada terapeuta,
	// 1 agendada, 1 confirmada.
	realizada := mk(2)
	env.AdminPOST(t, "/v1/admin/sessions/"+realizada+"/finish", map[string]any{"consider_cost": false}).Body.Close()

	falta := mk(4)
	env.AdminPOST(t, "/v1/admin/sessions/"+falta+"/no-show", nil).Body.Close()

	cpac := mk(6)
	env.AdminPOST(t, "/v1/admin/sessions/"+cpac+"/cancel", map[string]any{"canceled_by": "paciente"}).Body.Close()

	cter := mk(8)
	env.AdminPOST(t, "/v1/admin/sessions/"+cter+"/cancel", map[string]any{"canceled_by": "terapeuta"}).Body.Close()

	_ = mk(10) // agendada (fica agendada)

	conf := mk(12)
	env.AdminPOST(t, "/v1/admin/sessions/"+conf+"/confirm", nil).Body.Close()

	// Métricas (histórico completo).
	resp := env.AdminGET(t, "/v1/admin/patients/"+pid+"/session-metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session-metrics status = %d", resp.StatusCode)
	}
	var m struct {
		Counts             map[string]int `json:"counts"`
		Total              int            `json:"total"`
		Faturado           int64          `json:"faturado_realizadas"`
		TaxaComparecimento float64        `json:"taxa_comparecimento"`
		TaxaCancelamento   float64        `json:"taxa_cancelamento"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &m)

	if m.Total != 6 {
		t.Errorf("total = %d, quer 6", m.Total)
	}
	if m.Counts["realizadas"] != 1 {
		t.Errorf("realizadas = %d, quer 1", m.Counts["realizadas"])
	}
	if m.Counts["faltas"] != 1 {
		t.Errorf("faltas = %d, quer 1", m.Counts["faltas"])
	}
	if m.Counts["canceladas_paciente"] != 1 || m.Counts["canceladas_terapeuta"] != 1 {
		t.Errorf("canceladas por autor incorretas: %+v", m.Counts)
	}
	if m.Counts["agendadas_confirmadas"] != 2 {
		t.Errorf("agendadas/confirmadas = %d, quer 2", m.Counts["agendadas_confirmadas"])
	}
	// Comparecimento = 1 / (1 realizada + 1 falta + 1 cancelada paciente) = 1/3.
	if m.TaxaComparecimento < 0.33 || m.TaxaComparecimento > 0.34 {
		t.Errorf("comparecimento = %v, quer ~0.333", m.TaxaComparecimento)
	}
	// Faturado = débito da realizada (15000).
	if m.Faturado != 15000 {
		t.Errorf("faturado = %d, quer 15000", m.Faturado)
	}
}

// TestLocationSessionReport confere o relatório por local no período.
func TestLocationSessionReport(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Local presencial.
	lresp := env.AdminPOST(t, "/v1/admin/locations", map[string]any{
		"name": "Clínica Parceira", "modality": "presencial", "cost_period": "mensal",
	})
	var loc struct {
		ID string `json:"id"`
	}
	DecodeEnvelope(t, lresp).DataAs(t, &loc)

	pid := env.createPatient(t, "local-rel@example.com")
	env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": pid, "type": "pagamento_por_consulta", "amount": 20000,
	}).Body.Close()

	mk := func(hour int) string {
		start := time.Now().Add(time.Duration(hour) * time.Hour)
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "location_id": loc.ID, "modality": "presencial",
			"starts_at": start.Format(time.RFC3339), "ends_at": start.Add(time.Hour).Format(time.RFC3339),
			"status": "agendada",
		})
		var s struct {
			ID string `json:"id"`
		}
		DecodeEnvelope(t, resp).DataAs(t, &s)
		return s.ID
	}

	r1 := mk(2)
	env.AdminPOST(t, "/v1/admin/sessions/"+r1+"/finish", map[string]any{"consider_cost": false}).Body.Close()
	_ = mk(4) // agendada
	cpac := mk(6)
	env.AdminPOST(t, "/v1/admin/sessions/"+cpac+"/cancel", map[string]any{"canceled_by": "paciente"}).Body.Close()

	// Relatório do mês atual (default).
	resp := env.AdminGET(t, "/v1/admin/locations/"+loc.ID+"/session-report")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session-report status = %d", resp.StatusCode)
	}
	var rep struct {
		LocationName string         `json:"location_name"`
		Counts       map[string]int `json:"counts"`
		Total        int            `json:"total"`
		Faturado     int64          `json:"faturado_realizadas"`
	}
	DecodeEnvelope(t, resp).DataAs(t, &rep)

	if rep.LocationName != "Clínica Parceira" {
		t.Errorf("nome do local = %q", rep.LocationName)
	}
	if rep.Total != 3 {
		t.Errorf("total = %d, quer 3", rep.Total)
	}
	if rep.Counts["realizadas"] != 1 || rep.Counts["agendadas_confirmadas"] != 1 || rep.Counts["canceladas_paciente"] != 1 {
		t.Errorf("contagens incorretas: %+v", rep.Counts)
	}
	if rep.Faturado != 20000 {
		t.Errorf("faturado = %d, quer 20000", rep.Faturado)
	}
}
