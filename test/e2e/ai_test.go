package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// TestAIDesligadaRecusa (Fase 8 / R10.3): sem llm.gemini.api_key no ambiente, a
// IA está DESLIGADA (opt-in pela presença da chave). Disparar o resumo responde
// 422 (validação), nunca 500 — e nada é processado. O caminho ON (Gemini
// mockado) é coberto em unit no internal/app (fake Summarizer), já que o binário
// real não injeta um mock HTTP do Gemini e nenhum teste toca a API real.
func TestAIDesligadaRecusa(t *testing.T) {
	base := startServer(t) // sobe sem PSICOMAN_GEMINI_API_KEY

	// Paciente + sessão para ter um id de sessão plausível na rota.
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"IA Off","phone":"11999997777","email":"iaoff@exemplo.com"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)

	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"presencial","starts_at":"2026-05-10T14:00:00-03:00","ends_at":"2026-05-10T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("agendar sessão: status %d (%s)", status, body)
	}
	sid := dataID(t, body)

	// Disparo da IA com ela desligada: 422, sem 500.
	status, body = postJSON(t, base, "/v1/app/sessions/"+sid+"/ai-summary",
		`{"text":"conteúdo clínico qualquer"}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("IA off: quero 422, veio %d (%s)", status, body)
	}
	if !strings.Contains(body, `"error"`) {
		t.Errorf("resposta deveria ter envelope de erro: %s", body)
	}
}
