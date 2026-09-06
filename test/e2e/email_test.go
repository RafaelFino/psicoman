package e2e

import (
	"net/http"
	"testing"
)

func TestSendChargeEmail(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	debtID := env.debtFor(t, 25000)

	before := env.Google.EmailCount()
	resp := env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/send-email", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send-email status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	if env.Google.EmailCount() != before+1 {
		t.Errorf("emails enviados = %d, quer %d", env.Google.EmailCount(), before+1)
	}
	last := env.Google.SentEmails[len(env.Google.SentEmails)-1]
	if last.Subject == "" || last.To == "" {
		t.Errorf("email incompleto: %+v", last)
	}
}

func TestSendTemplateEmail(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "tplemail@example.com")
	resp := env.AdminPOST(t, "/v1/admin/templates", map[string]any{
		"name": "Orientações", "body_md": "# Orientações\n\nCompareça com **10 minutos** de antecedência.",
	})
	ce := DecodeEnvelope(t, resp)
	var tpl struct {
		ID string `json:"id"`
	}
	ce.DataAs(t, &tpl)

	before := env.Google.EmailCount()
	resp = env.AdminPOST(t, "/v1/admin/templates/"+tpl.ID+"/send", map[string]any{"patient_id": pid})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send template status = %d, quer 201", resp.StatusCode)
	}
	resp.Body.Close()

	if env.Google.EmailCount() != before+1 {
		t.Errorf("emails de template enviados = %d, quer %d", env.Google.EmailCount(), before+1)
	}
}
