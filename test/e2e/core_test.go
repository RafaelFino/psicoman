package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestLocationsOriginsContrato (R1.7): CRUD de locais + disponibilidade +
// origens via o contrato /v1/app.
func TestLocationsOriginsContrato(t *testing.T) {
	base := startServer(t)

	// Local.
	status, body := postJSON(t, base, "/v1/app/locations",
		`{"name":"Consultório Centro","modality":"presencial","cost_cents":80000,"cost_period":"mensal"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar local: status %d (%s)", status, body)
	}
	locID := dataID(t, body)

	// Disponibilidade no local.
	status, body = postJSON(t, base, "/v1/app/locations/"+locID+"/availability",
		`{"weekday":2,"start_time":"08:00","end_time":"12:00","capacity":1}`)
	if status != http.StatusCreated {
		t.Fatalf("add disponibilidade: status %d (%s)", status, body)
	}

	// Lista disponibilidades.
	status, body = getJSONApp(t, base, "/v1/app/locations/"+locID+"/availability")
	if status != http.StatusOK {
		t.Fatalf("listar disponibilidade: status %d (%s)", status, body)
	}

	// Modalidade inválida → 422.
	status, _ = postJSON(t, base, "/v1/app/locations",
		`{"name":"X","modality":"hibrido","cost_period":"mensal"}`)
	if status != http.StatusUnprocessableEntity {
		t.Errorf("modalidade inválida: quero 422, veio %d", status)
	}

	// Origem.
	status, body = postJSON(t, base, "/v1/app/origins", `{"name":"Doctoralia"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar origem: status %d (%s)", status, body)
	}
	status, body = getJSONApp(t, base, "/v1/app/origins")
	if status != http.StatusOK {
		t.Fatalf("listar origens: status %d (%s)", status, body)
	}
}

// TestDebitoPorSessaoPontaAPonta (R1.9, jornada principal da Fase 4): cadastra
// paciente → cria plano POR CONSULTA via API → cria sessão → finaliza →
// verifica que o DÉBITO foi gerado automaticamente (hook de billing) → gera a
// cobrança em PDF. Prova que o débito por sessão funciona ponta a ponta SÓ pela
// API (sem seed via superuser).
func TestDebitoPorSessaoPontaAPonta(t *testing.T) {
	base := startServer(t)

	// 1. Paciente.
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Jornada E2E","phone":"11999990777","email":"jornada@exemplo.com","cpf":"39053344705"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)

	// 2. Plano POR CONSULTA vigente (valor por sessão). starts_at antes da sessão.
	status, body = postJSON(t, base, "/v1/app/patients/"+pid+"/plans",
		`{"type":"pagamento_por_consulta","amount_cents":18000,"starts_at":"2026-01-01T00:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar plano: status %d (%s)", status, body)
	}

	// 3. Sessão.
	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-06-10T14:00:00-03:00","ends_at":"2026-06-10T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar sessão: status %d (%s)", status, body)
	}
	sid := dataID(t, body)

	// 4. Finaliza a sessão (dispara o hook de billing → gera o débito).
	status, body = postJSON(t, base, "/v1/app/sessions/"+sid+"/finish", `{"bill":true}`)
	if status != http.StatusOK {
		t.Fatalf("finalizar sessão: status %d (%s)", status, body)
	}

	// 5. O débito foi gerado automaticamente (sem seed): lista os débitos do paciente.
	status, body = getJSONApp(t, base, "/v1/app/patients/"+pid+"/debts")
	if status != http.StatusOK {
		t.Fatalf("listar débitos: status %d (%s)", status, body)
	}
	var env struct {
		Data []struct {
			ID          string `json:"id"`
			AmountCents int64  `json:"amount_cents"`
			SessionID   string `json:"session_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decodificar débitos: %v (%s)", err, body)
	}
	if len(env.Data) != 1 {
		t.Fatalf("esperava 1 débito gerado pela sessão, veio %d: %s", len(env.Data), body)
	}
	if env.Data[0].AmountCents != 18000 {
		t.Errorf("valor do débito deveria ser 18000 (do plano), veio %d", env.Data[0].AmountCents)
	}
	if env.Data[0].SessionID != sid {
		t.Errorf("débito deveria referenciar a sessão %s, veio %q", sid, env.Data[0].SessionID)
	}
	debtID := env.Data[0].ID

	// 6. Gera a cobrança em PDF do débito (fecha o ciclo).
	status, body = postJSON(t, base, "/v1/app/debts/"+debtID+"/invoice", ``)
	if status != http.StatusCreated {
		t.Fatalf("gerar cobrança: status %d (%s)", status, body)
	}

	// 7. Paga o débito COM comprovante (base64) — o pagamento fica pago (R1.9).
	// "comprovante" em base64 de um conteúdo arbitrário.
	const receiptB64 = "JVBERi0xLjQgY29tcHJvdmFudGU=" // "%PDF-1.4 comprovante"
	status, body = postJSON(t, base, "/v1/app/debts/"+debtID+"/pay",
		`{"amount_cents":18000,"method":"pix","receipt_base64":"`+receiptB64+`","receipt_mime":"application/pdf"}`)
	if status != http.StatusOK {
		t.Fatalf("pagar com comprovante: status %d (%s)", status, body)
	}
	var payEnv struct {
		Data struct {
			Debt struct {
				Status string `json:"status"`
			} `json:"debt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &payEnv); err != nil {
		t.Fatalf("decodificar pagamento: %v (%s)", err, body)
	}
	if payEnv.Data.Debt.Status != "pago" {
		t.Errorf("após pagar o total, débito deveria estar pago, veio %q", payEnv.Data.Debt.Status)
	}
}

// TestPlanSemValidacao422 (R1.9): plano fechado sem valor → 422.
func TestPlanSemValidacao422(t *testing.T) {
	base := startServer(t)
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Plano Invalido","phone":"11999990778","email":"planoinv@exemplo.com"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)
	status, _ = postJSON(t, base, "/v1/app/patients/"+pid+"/plans",
		`{"type":"plano_fechado_mensal","amount_cents":0,"due_day":10,"starts_at":"2026-01-01T00:00:00-03:00"}`)
	if status != http.StatusUnprocessableEntity {
		t.Errorf("plano fechado sem valor: quero 422, veio %d", status)
	}
}
