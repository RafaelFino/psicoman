package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// portalPost faz um POST no portal com a sessão do paciente (Bearer).
func portalPost(t *testing.T, base, path, token, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// registerAndLogin cadastra pelo portal e devolve (patientID, token).
func registerAndLogin(t *testing.T, base, name, phone, email string) (string, string) {
	t.Helper()
	const pass = "senha-forte-123"
	status, body := postJSON(t, base, "/v1/portal/register",
		`{"name":"`+name+`","phone":"`+phone+`","email":"`+email+`","password":"`+pass+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("cadastro %s: %d (%s)", email, status, body)
	}
	return dataID(t, body), portalLogin(t, base, email, pass)
}

const (
	slotA = `"slot_start":"2026-07-01T14:00:00-03:00","slot_end":"2026-07-01T15:00:00-03:00"`
	slotB = `"slot_start":"2026-07-02T14:00:00-03:00","slot_end":"2026-07-02T15:00:00-03:00"`
)

// TestPedidoDeAgendamentoJornada (R1.8): pendente é barrado (403); aprovado
// pede; o terapeuta vê a fila, confirma (cria sessão) ou recusa; pacientes não
// enxergam pedidos uns dos outros.
func TestPedidoDeAgendamentoJornada(t *testing.T) {
	base := startServer(t)

	pidA, tokA := registerAndLogin(t, base, "Paciente A", "11999990301", "a-appt@exemplo.com")
	pidB, tokB := registerAndLogin(t, base, "Paciente B", "11999990302", "b-appt@exemplo.com")

	// NEGADO: paciente pendente → 403 (gate); sem sessão → 401.
	if st, b := portalPost(t, base, "/v1/portal/appointment-requests", tokA, `{`+slotA+`}`); st != http.StatusForbidden {
		t.Fatalf("pendente deveria ser 403, veio %d (%s)", st, b)
	}
	if st, _ := portalPost(t, base, "/v1/portal/appointment-requests", "", `{`+slotA+`}`); st != http.StatusUnauthorized {
		t.Errorf("sem sessão deveria ser 401, veio %d", st)
	}

	// Aprova A e B.
	for _, id := range []string{pidA, pidB} {
		if st, b := postJSON(t, base, "/v1/app/patients/"+id+"/approve", ``); st != http.StatusOK {
			t.Fatalf("aprovar %s: %d (%s)", id, st, b)
		}
	}

	// Validações do pedido (aprovado): corpo ruim 400; slot invertido 422; nota longa 422.
	if st, _ := portalPost(t, base, "/v1/portal/appointment-requests", tokA, `{isso nao e json`); st != http.StatusBadRequest {
		t.Errorf("JSON ruim: quero 400, veio %d", st)
	}
	if st, _ := portalPost(t, base, "/v1/portal/appointment-requests", tokA,
		`{"slot_start":"2026-07-01T15:00:00-03:00","slot_end":"2026-07-01T14:00:00-03:00"}`); st != http.StatusUnprocessableEntity {
		t.Errorf("slot invertido: quero 422, veio %d", st)
	}
	if st, _ := portalPost(t, base, "/v1/portal/appointment-requests", tokA,
		`{`+slotA+`,"note":"`+strings.Repeat("x", 501)+`"}`); st != http.StatusUnprocessableEntity {
		t.Errorf("nota longa: quero 422, veio %d", st)
	}
	if st, _ := portalPost(t, base, "/v1/portal/appointment-requests", tokA,
		`{`+slotA+`,"location_id":"nao-existe"}`); st != http.StatusUnprocessableEntity {
		t.Errorf("local inexistente: quero 422, veio %d", st)
	}

	// PERMITIDO: A pede dois horários.
	st, body := portalPost(t, base, "/v1/portal/appointment-requests", tokA, `{`+slotA+`,"note":"prefiro à tarde"}`)
	if st != http.StatusCreated {
		t.Fatalf("A pede 1: %d (%s)", st, body)
	}
	reqA1 := dataID(t, body)
	st, body = portalPost(t, base, "/v1/portal/appointment-requests", tokA, `{`+slotB+`}`)
	if st != http.StatusCreated {
		t.Fatalf("A pede 2: %d (%s)", st, body)
	}
	reqA2 := dataID(t, body)

	// Isolamento: A vê 2, B vê 0.
	_, body = portalGet(t, base, "/v1/portal/appointment-requests", tokA)
	if c := strings.Count(body, `"status":"pendente"`); c != 2 {
		t.Errorf("A deveria ver 2 pendentes, vê %d (%s)", c, body)
	}
	_, body = portalGet(t, base, "/v1/portal/appointment-requests", tokB)
	if strings.Contains(body, reqA1) || strings.Contains(body, reqA2) {
		t.Errorf("VAZAMENTO: B enxerga pedidos de A: %s", body)
	}

	// Terapeuta: fila com 2 pendentes.
	st, body = getJSONApp(t, base, "/v1/app/appointment-requests")
	if st != http.StatusOK || strings.Count(body, `"status":"pendente"`) != 2 {
		t.Fatalf("fila do terapeuta: %d (%s)", st, body)
	}

	// Confirma o 1º → cria sessão agendada; pedido sai da fila.
	st, body = postJSON(t, base, "/v1/app/appointment-requests/"+reqA1+"/confirm", ``)
	if st != http.StatusCreated {
		t.Fatalf("confirmar: %d (%s)", st, body)
	}
	var sess struct {
		Data struct {
			ID        string `json:"id"`
			PatientID string `json:"patient_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &sess); err != nil || sess.Data.PatientID != pidA || sess.Data.Status != "agendada" {
		t.Fatalf("sessão do pedido incorreta: %s (err=%v)", body, err)
	}
	// Reconfirmar → 409; a sessão aparece só para A no portal.
	if st, _ := postJSON(t, base, "/v1/app/appointment-requests/"+reqA1+"/confirm", ``); st != http.StatusConflict {
		t.Errorf("reconfirmar: quero 409, veio %d", st)
	}
	_, body = portalGet(t, base, "/v1/portal/sessions", tokA)
	if !strings.Contains(body, sess.Data.ID) {
		t.Errorf("A deveria ver a sessão criada: %s", body)
	}
	_, body = portalGet(t, base, "/v1/portal/sessions", tokB)
	if strings.Contains(body, sess.Data.ID) {
		t.Errorf("VAZAMENTO: B vê a sessão de A: %s", body)
	}

	// Recusa o 2º; depois não confirma mais (409) e a fila esvazia.
	if st, b := postJSON(t, base, "/v1/app/appointment-requests/"+reqA2+"/refuse", ``); st != http.StatusOK {
		t.Fatalf("recusar: %d (%s)", st, b)
	}
	if st, _ := postJSON(t, base, "/v1/app/appointment-requests/"+reqA2+"/confirm", ``); st != http.StatusConflict {
		t.Errorf("confirmar recusado: quero 409, veio %d", st)
	}
	if st, _ := postJSON(t, base, "/v1/app/appointment-requests/nao-existe/confirm", ``); st != http.StatusNotFound {
		t.Errorf("confirmar inexistente: quero 404, veio %d", st)
	}
	_, body = getJSONApp(t, base, "/v1/app/appointment-requests")
	if strings.Contains(body, `"status":"pendente"`) {
		t.Errorf("fila deveria estar vazia: %s", body)
	}
	// Status final visível ao paciente.
	_, body = portalGet(t, base, "/v1/portal/appointment-requests", tokA)
	if !strings.Contains(body, `"status":"confirmado"`) || !strings.Contains(body, `"status":"recusado"`) {
		t.Errorf("A deveria ver confirmado e recusado: %s", body)
	}

	// Rotas do app exigem identidade do Pangolin (sem header → 401).
	resp, err := http.Get(base + "/v1/app/appointment-requests")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("app sem header: quero 401, veio %d", resp.StatusCode)
	}
}
