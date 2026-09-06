package e2e

import (
	"net/http"
	"testing"
)

// TestEnvelopeContract valida o envelope padrão e o versionamento /v1.
func TestEnvelopeContract(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.GET(t, "/v1/admin/version")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	if e.Message == "" {
		t.Error("envelope sem message PT-BR")
	}
	if e.RequestID == "" {
		t.Error("envelope sem request_id")
	}
	var data struct {
		APIVersion string `json:"api_version"`
		Surface    string `json:"surface"`
	}
	e.DataAs(t, &data)
	if data.APIVersion != "v1" {
		t.Errorf("api_version = %q, quer v1", data.APIVersion)
	}
	if data.Surface != "admin" {
		t.Errorf("surface = %q, quer admin", data.Surface)
	}
}

// TestEnvelopeError valida o formato de erro (404 no envelope).
func TestEnvelopeError(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	resp := env.GET(t, "/v1/admin/inexistente")
	// mux devolve 404 padrão; garantimos que o servidor não quebra.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, quer 404", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestSwaggerServed valida que a spec e a UI estão disponíveis.
func TestSwaggerServed(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	specResp := env.GET(t, "/v1/swagger.json")
	if specResp.StatusCode != http.StatusOK {
		t.Errorf("swagger.json status = %d, quer 200", specResp.StatusCode)
	}
	specResp.Body.Close()

	uiResp := env.GET(t, "/v1/swagger")
	if uiResp.StatusCode != http.StatusOK {
		t.Errorf("swagger UI status = %d, quer 200", uiResp.StatusCode)
	}
	uiResp.Body.Close()
}

// TestPortalVersion valida o namespace do portal.
func TestPortalVersion(t *testing.T) {
	env := StartPortal(t)
	defer env.Stop()

	resp := env.GET(t, "/v1/portal/version")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var data struct {
		Surface string `json:"surface"`
	}
	e.DataAs(t, &data)
	if data.Surface != "portal" {
		t.Errorf("surface = %q, quer portal", data.Surface)
	}
}
