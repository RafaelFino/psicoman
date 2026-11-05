package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestCustosEROIJornada (R1.5/R1.6): categoria + item de custo ligado a uma
// origem, local com custo por sessão, sessão finalizada com consider_cost, e os
// relatórios de custo, ROI e de sessões por local.
func TestCustosEROIJornada(t *testing.T) {
	base := startServer(t)

	// Origem + categoria de plataforma + item de custo mensal ligado à origem.
	status, body := postJSON(t, base, "/v1/app/origins", `{"name":"Doctoralia"}`)
	if status != http.StatusCreated {
		t.Fatalf("origem: %d (%s)", status, body)
	}
	originID := dataID(t, body)

	status, body = postJSON(t, base, "/v1/app/cost-categories", `{"kind":"plataforma","name":"Plataformas"}`)
	if status != http.StatusCreated {
		t.Fatalf("categoria: %d (%s)", status, body)
	}
	catID := dataID(t, body)

	status, body = postJSON(t, base, "/v1/app/cost-items",
		`{"category_id":"`+catID+`","name":"Plano Doctoralia","amount_cents":30000,"period":"mensal","origin_id":"`+originID+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("item de custo: %d (%s)", status, body)
	}
	itemID := dataID(t, body)

	status, body = getJSONApp(t, base, "/v1/app/cost-items")
	if status != http.StatusOK || !strings.Contains(body, itemID) {
		t.Fatalf("listar itens: %d (%s)", status, body)
	}
	status, body = getJSONApp(t, base, "/v1/app/cost-categories")
	if status != http.StatusOK || !strings.Contains(body, catID) {
		t.Fatalf("listar categorias: %d (%s)", status, body)
	}

	// Validação: categoria inválida e item sem nome → 422.
	if st, _ := postJSON(t, base, "/v1/app/cost-categories", `{"kind":"xpto","name":"X"}`); st != http.StatusUnprocessableEntity {
		t.Errorf("categoria inválida: quero 422, veio %d", st)
	}
	if st, _ := postJSON(t, base, "/v1/app/cost-items", `{"name":"","period":"mensal"}`); st != http.StatusUnprocessableEntity {
		t.Errorf("item sem nome: quero 422, veio %d", st)
	}

	// Local com custo por sessão; paciente (com origem) + plano por consulta.
	status, body = postJSON(t, base, "/v1/app/locations",
		`{"name":"Clínica Parceira","modality":"presencial","cost_cents":5000,"cost_period":"por_sessao"}`)
	if status != http.StatusCreated {
		t.Fatalf("local: %d (%s)", status, body)
	}
	locID := dataID(t, body)

	status, body = postJSON(t, base, "/v1/app/patients",
		`{"name":"Custos E2E","phone":"11999990555","email":"custos@exemplo.com","origin_id":"`+originID+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("paciente: %d (%s)", status, body)
	}
	pid := dataID(t, body)
	if st, b := postJSON(t, base, "/v1/app/patients/"+pid+"/plans",
		`{"type":"pagamento_por_consulta","amount_cents":18000,"starts_at":"2026-01-01T00:00:00-03:00"}`); st != http.StatusCreated {
		t.Fatalf("plano: %d (%s)", st, b)
	}

	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","location_id":"`+locID+`","modality":"presencial","starts_at":"2026-06-10T14:00:00-03:00","ends_at":"2026-06-10T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("sessão: %d (%s)", status, body)
	}
	sid := dataID(t, body)
	if st, b := postJSON(t, base, "/v1/app/sessions/"+sid+"/finish", `{"bill":true,"consider_cost":true}`); st != http.StatusOK {
		t.Fatalf("finalizar: %d (%s)", st, b)
	}

	// Custo da sessão: direto, 5000.
	status, body = getJSONApp(t, base, "/v1/app/sessions/"+sid+"/cost")
	if status != http.StatusOK {
		t.Fatalf("custo da sessão: %d (%s)", status, body)
	}
	var sc struct {
		Data struct {
			AmountCents int64  `json:"amount_cents"`
			Method      string `json:"method"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &sc); err != nil || sc.Data.AmountCents != 5000 || sc.Data.Method != "direto" {
		t.Errorf("custo da sessão incorreto: %s (err=%v)", body, err)
	}

	// Sessão sem custo atribuído → 404.
	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-06-11T14:00:00-03:00","ends_at":"2026-06-11T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("sessão 2: %d (%s)", status, body)
	}
	if st, _ := getJSONApp(t, base, "/v1/app/sessions/"+dataID(t, body)+"/cost"); st != http.StatusNotFound {
		t.Errorf("sessão sem custo: quero 404, veio %d", st)
	}

	// Relatório de custos do período.
	status, body = getJSONApp(t, base, "/v1/app/reports/costs?de=2026-06-01&ate=2026-06-30")
	if status != http.StatusOK || !strings.Contains(body, "by_kind_cents") || !strings.Contains(body, "by_patient_cents") {
		t.Fatalf("relatório de custos: %d (%s)", status, body)
	}
	if st, _ := getJSONApp(t, base, "/v1/app/reports/costs?de=2026-06-30&ate=2026-06-01"); st != http.StatusUnprocessableEntity {
		t.Errorf("período invertido: quero 422, veio %d", st)
	}
	if st, _ := getJSONApp(t, base, "/v1/app/reports/costs"); st != http.StatusBadRequest {
		t.Errorf("sem período: quero 400, veio %d", st)
	}

	// ROI inclui o custo da plataforma da origem.
	status, body = getJSONApp(t, base, "/v1/app/reports/roi?de=2026-06-01&ate=2026-06-30")
	if status != http.StatusOK || !strings.Contains(body, originID) {
		t.Fatalf("ROI: %d (%s)", status, body)
	}

	// Relatório de sessões por local: 1 realizada, faturado 18000.
	status, body = getJSONApp(t, base, "/v1/app/locations/"+locID+"/session-report?de=2026-06-01&ate=2026-06-30")
	if status != http.StatusOK {
		t.Fatalf("session-report: %d (%s)", status, body)
	}
	var rep struct {
		Data struct {
			LocationName string `json:"location_name"`
			Report       struct {
				Total        int   `json:"total"`
				FaturadoReal int64 `json:"faturado_realizadas"`
			} `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &rep); err != nil {
		t.Fatalf("decodificar session-report: %v (%s)", err, body)
	}
	if rep.Data.LocationName != "Clínica Parceira" || rep.Data.Report.Total != 1 || rep.Data.Report.FaturadoReal != 18000 {
		t.Errorf("session-report incorreto: %s", body)
	}

	// Default (mês corrente) responde 200; local inexistente → 404.
	if st, _ := getJSONApp(t, base, "/v1/app/locations/"+locID+"/session-report"); st != http.StatusOK {
		t.Errorf("session-report sem período: quero 200, veio %d", st)
	}
	if st, _ := getJSONApp(t, base, "/v1/app/locations/nao-existe/session-report"); st != http.StatusNotFound {
		t.Errorf("local inexistente: quero 404, veio %d", st)
	}

	// Remover o item de custo.
	if st, _ := deleteReq(t, base, "/v1/app/cost-items/"+itemID); st != http.StatusOK {
		t.Errorf("remover item: quero 200, veio %d", st)
	}
}
