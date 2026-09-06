package e2e

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"
)

// debtFor gera um débito de valor `amount` para um paciente novo e devolve (debtID).
func (e *Env) debtFor(t *testing.T, amount int64) string {
	t.Helper()
	pid := e.finishBilledSession(t, "pagamento_por_consulta", amount)
	resp := e.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	env := DecodeEnvelope(t, resp)
	var debts []map[string]any
	env.DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	return debts[0]["id"].(string)
}

func TestDebtFullPayment(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	debtID := env.debtFor(t, 20000)

	// Pagamento total → status pago.
	resp := env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{
		"amount": 20000, "method": "pix",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pay status = %d, quer 201", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var res struct {
		DebtStatus string `json:"debt_status"`
	}
	e.DataAs(t, &res)
	if res.DebtStatus != "pago" {
		t.Errorf("status = %q, quer pago", res.DebtStatus)
	}
}

func TestDebtPartialThenFull(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	debtID := env.debtFor(t, 30000)

	// Parcial.
	resp := env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 10000})
	e := DecodeEnvelope(t, resp)
	var r1 struct {
		DebtStatus string `json:"debt_status"`
	}
	e.DataAs(t, &r1)
	if r1.DebtStatus != "parcial" {
		t.Errorf("status após parcial = %q, quer parcial", r1.DebtStatus)
	}

	// Restante → pago.
	resp = env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 20000})
	e2 := DecodeEnvelope(t, resp)
	var r2 struct {
		DebtStatus string `json:"debt_status"`
	}
	e2.DataAs(t, &r2)
	if r2.DebtStatus != "pago" {
		t.Errorf("status após quitar = %q, quer pago", r2.DebtStatus)
	}

	// Lista de pagamentos tem 2.
	resp = env.AdminGET(t, "/v1/admin/debts/"+debtID+"/payments")
	le := DecodeEnvelope(t, resp)
	var pays []map[string]any
	le.DataAs(t, &pays)
	if len(pays) != 2 {
		t.Errorf("pagamentos = %d, quer 2", len(pays))
	}
}

func TestPaymentReceiptAttachment(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	debtID := env.debtFor(t, 15000)

	// Paga.
	resp := env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 15000, "method": "transferencia"})
	e := DecodeEnvelope(t, resp)
	var res struct {
		Payment struct {
			ID string `json:"id"`
		} `json:"payment"`
	}
	e.DataAs(t, &res)
	if res.Payment.ID == "" {
		t.Fatal("pagamento sem id")
	}

	// Anexa comprovante (multipart com campo file + debt_id).
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "comprovante.pdf")
	part.Write([]byte("%PDF-1.4 comprovante"))
	mw.WriteField("debt_id", debtID)
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, env.BaseURL+"/v1/admin/payments/"+res.Payment.ID+"/receipt", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Pangolin-Email", testAdminEmail)
	req.Header.Set("X-Pangolin-Secret", testAdminSecret)
	rresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("anexar comprovante: %v", err)
	}
	defer rresp.Body.Close()
	if rresp.StatusCode != http.StatusCreated {
		t.Errorf("receipt status = %d, quer 201", rresp.StatusCode)
	}
}
