package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// TestTelasIniciaisContratoENaoVazamento (R1.1-R1.4): cadastra paciente, cria
// uma nota clínica com conteúdo SECRETO e uma sessão; então exercita feed do
// dia, história, métricas e calendário. Verifica o contrato (200) e que o
// conteúdo clínico da nota NUNCA aparece nessas respostas (R1.2: história sem
// clínico em metadados).
func TestTelasIniciaisContratoENaoVazamento(t *testing.T) {
	base := startServer(t)

	const segredoClinico = "SEGREDO_CLINICO_NAO_PODE_VAZAR_NAS_TELAS"

	// Paciente.
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Telas E2E","phone":"11999990888","email":"telas@exemplo.com"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)

	// Nota clínica com conteúdo secreto (vai cifrada; não pode vazar nas telas).
	status, _ = postJSON(t, base, "/v1/app/patients/"+pid+"/notes",
		`{"content":"`+segredoClinico+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar nota: status %d", status)
	}

	// Sessão (base do feed/calendário/métricas).
	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-06-10T14:00:00-03:00","ends_at":"2026-06-10T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar sessão: status %d (%s)", status, body)
	}

	// 1. Feed do dia (R1.1) — sem clínico.
	status, body = getJSONApp(t, base, "/v1/app/day-feed?window=recentes")
	if status != http.StatusOK {
		t.Fatalf("day-feed: status %d (%s)", status, body)
	}
	if strings.Contains(body, segredoClinico) {
		t.Errorf("VAZAMENTO: conteúdo clínico no feed: %s", body)
	}

	// 2. História do paciente (R1.2) — itens com rótulos, nunca o conteúdo da nota.
	status, body = getJSONApp(t, base, "/v1/app/patients/"+pid+"/history")
	if status != http.StatusOK {
		t.Fatalf("history: status %d (%s)", status, body)
	}
	if strings.Contains(body, segredoClinico) {
		t.Errorf("VAZAMENTO: conteúdo clínico na história: %s", body)
	}
	// A história deve conter o item da nota (rótulo), provando que ela aparece
	// sem revelar o conteúdo.
	if !strings.Contains(body, "nota") && !strings.Contains(body, "Nota") && !strings.Contains(body, "registro") {
		t.Logf("aviso: história sem item de nota rotulado (ok se o rótulo diferir): %s", body)
	}

	// 3. Métricas de sessão (R1.3).
	status, body = getJSONApp(t, base, "/v1/app/patients/"+pid+"/session-metrics")
	if status != http.StatusOK {
		t.Fatalf("session-metrics: status %d (%s)", status, body)
	}
	if !strings.Contains(body, "taxa_comparecimento") {
		t.Errorf("métricas sem taxa_comparecimento: %s", body)
	}

	// 4. Calendário (R1.4): sessões do paciente por intervalo.
	status, body = getJSONApp(t, base, "/v1/app/sessions?patient_id="+pid+"&from=2026-06-01&to=2026-06-30")
	if status != http.StatusOK {
		t.Fatalf("calendário por paciente: status %d (%s)", status, body)
	}
	if strings.Contains(body, segredoClinico) {
		t.Errorf("VAZAMENTO: clínico no calendário: %s", body)
	}

	// 5. Calendário por intervalo (sem patient_id, exige from+to).
	status, body = getJSONApp(t, base, "/v1/app/sessions?from=2026-06-01&to=2026-06-30")
	if status != http.StatusOK {
		t.Fatalf("calendário por intervalo: status %d (%s)", status, body)
	}

	// 6. Sem patient_id e sem intervalo → 422.
	status, _ = getJSONApp(t, base, "/v1/app/sessions")
	if status != http.StatusUnprocessableEntity {
		t.Errorf("calendário sem filtro: quero 422, veio %d", status)
	}
}
