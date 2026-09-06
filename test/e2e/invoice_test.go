package e2e

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestInvoicePDFGeneration(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	// Gera um débito via finalização de sessão faturável (plano por consulta).
	pid := env.finishBilledSession(t, "pagamento_por_consulta", 15000)

	// Recupera o débito.
	resp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	e := DecodeEnvelope(t, resp)
	var debts []map[string]any
	e.DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	debtID := debts[0]["id"].(string)

	// Gera o PDF.
	resp = env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pdf", map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("gerar PDF status = %d, quer 201", resp.StatusCode)
	}
	ge := DecodeEnvelope(t, resp)
	var gen struct {
		GEDFileID string `json:"ged_file_id"`
	}
	ge.DataAs(t, &gen)
	if gen.GEDFileID == "" {
		t.Fatal("PDF não vinculado ao GED")
	}

	// Baixa o PDF.
	resp = env.AdminGET(t, "/v1/admin/debts/"+debtID+"/pdf")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("baixar PDF status = %d, quer 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("content-type = %q, quer application/pdf", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.HasPrefix(body, []byte("%PDF-1.4")) {
		t.Error("resposta não é um PDF válido")
	}

	// O débito agora referencia o arquivo do GED.
	resp = env.AdminGET(t, "/v1/admin/debts/"+debtID)
	de := DecodeEnvelope(t, resp)
	var debt struct {
		PDFFileID string `json:"pdf_file_id"`
	}
	de.DataAs(t, &debt)
	if debt.PDFFileID != gen.GEDFileID {
		t.Errorf("pdf_file_id = %q, quer %q", debt.PDFFileID, gen.GEDFileID)
	}
}

func TestInvoicePDFOnDemand(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.finishBilledSession(t, "pagamento_por_consulta", 8000)
	resp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	e := DecodeEnvelope(t, resp)
	var debts []map[string]any
	e.DataAs(t, &debts)
	debtID := debts[0]["id"].(string)

	// GET direto (sem POST prévio) deve gerar on-demand.
	resp = env.AdminGET(t, "/v1/admin/debts/"+debtID+"/pdf")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PDF on-demand status = %d, quer 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Error("PDF on-demand inválido")
	}
}
