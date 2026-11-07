package api

import (
	"net/http"
	"strings"
	"testing"
)

// pedirHorario faz POST /v1/portal/appointment-requests com um slot válido.
func pedirHorario(c *apiClient, hdr map[string]string) apiResp {
	return c.do(http.MethodPost, "/v1/portal/appointment-requests",
		`{"slot_start":"2026-12-10T14:00:00-03:00","slot_end":"2026-12-10T15:00:00-03:00","note":"teste"}`, hdr)
}

// TestPortalAgendamentoRateLimit429 (R7.1): 10 pedidos/h por paciente da
// sessão. O 11º é 429 com envelope e Retry-After; outro paciente não é afetado.
func TestPortalAgendamentoRateLimit429(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)

	const emailA, emailB = "limite-a@exemplo.com", "limite-b@exemplo.com"
	idA := registerPortal(t, c, "Paciente A", "11999990201", emailA)
	idB := registerPortal(t, c, "Paciente B", "11999990202", emailB)
	approvePatient(t, c, idA)
	approvePatient(t, c, idB)
	hdrA := portalAuthHeader(t, app, emailA)
	hdrB := portalAuthHeader(t, app, emailB)

	// Dentro do limite: nenhuma tentativa pode ser 429 (as respostas podem ser
	// 201/409/422 conforme a regra de negócio — o limitador conta todas).
	for i := 1; i <= appointmentLimitPerHourForTest; i++ {
		if res := pedirHorario(c, hdrA); res.status == http.StatusTooManyRequests {
			t.Fatalf("pedido %d (dentro do limite) não pode ser 429: %s", i, res.body)
		}
	}

	// 11º: 429, com envelope.
	res := pedirHorario(c, hdrA)
	if res.status != http.StatusTooManyRequests {
		t.Fatalf("11º pedido: quero 429, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"limite_excedido"`) {
		t.Errorf("envelope sem code limite_excedido: %s", res.body)
	}

	// Outro paciente NÃO é afetado (chave por paciente da sessão).
	if res := pedirHorario(c, hdrB); res.status == http.StatusTooManyRequests {
		t.Errorf("outro paciente não deveria ser limitado: %s", res.body)
	}

	// NEGADO sem sessão continua 401 (o limite não mascara a autenticação).
	if res := pedirHorario(c, nil); res.status != http.StatusUnauthorized {
		t.Errorf("sem sessão: quero 401, veio %d (%s)", res.status, res.body)
	}
}

// appointmentLimitPerHourForTest espelha a constante do portal (10/h) — o
// pacote api não importa constantes internas de api/portal.
const appointmentLimitPerHourForTest = 10
