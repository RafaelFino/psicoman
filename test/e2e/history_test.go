package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestPatientHistory exercita o contrato de GET /patients/{id}/history com um
// histórico variado: cadastro → sessão → confirmar → finalizar (gera débito) →
// pagar → anexar arquivo → remarcar → cancelar outra sessão. O feed deve trazer
// todos os tipos, ordenado do mais recente ao mais antigo.
func TestPatientHistory(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "historia@example.com")

	// Plano por sessão (para o finish gerar débito automaticamente).
	resp := env.AdminPOST(t, "/v1/admin/plans", map[string]any{
		"patient_id": pid, "type": "pagamento_por_consulta", "amount": 18000,
	})
	resp.Body.Close()

	mkSession := func(offsetHours int) string {
		start := time.Now().Add(time.Duration(offsetHours) * time.Hour)
		resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
			"patient_id": pid, "modality": "online",
			"starts_at": start.Format(time.RFC3339),
			"ends_at":   start.Add(time.Hour).Format(time.RFC3339),
			"status":    "agendada",
		})
		var s struct {
			ID string `json:"id"`
		}
		DecodeEnvelope(t, resp).DataAs(t, &s)
		return s.ID
	}

	// Sessão 1: confirmar → finalizar (gera débito) → pagar.
	s1 := mkSession(2)
	env.AdminPOST(t, "/v1/admin/sessions/"+s1+"/confirm", nil).Body.Close()
	fresp := env.AdminPOST(t, "/v1/admin/sessions/"+s1+"/finish", map[string]any{"consider_cost": false})
	if fresp.StatusCode != http.StatusOK {
		t.Fatalf("finish status = %d", fresp.StatusCode)
	}
	fresp.Body.Close()

	// Débito gerado → pagar.
	dresp := env.AdminGET(t, "/v1/admin/debts?patient_id="+pid)
	var debts []map[string]any
	DecodeEnvelope(t, dresp).DataAs(t, &debts)
	if len(debts) != 1 {
		t.Fatalf("débitos = %d, quer 1", len(debts))
	}
	debtID := debts[0]["id"].(string)
	env.AdminPOST(t, "/v1/admin/debts/"+debtID+"/pay", map[string]any{"amount": 18000, "method": "pix"}).Body.Close()

	// Anexa arquivo ao paciente.
	up := env.uploadFile(t, "/v1/admin/patients/"+pid+"/files", "laudo.txt", []byte("conteúdo clínico"))
	if up.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", up.StatusCode)
	}
	up.Body.Close()

	// Registro clínico vinculado à sessão 1 + nota livre.
	env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"session_id": s1, "content": "relatório da sessão",
	}).Body.Close()
	env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"content": "anotação livre",
	}).Body.Close()

	// Sessão 2: remarcar (gera audit session.reschedule).
	s2 := mkSession(48)
	newStart := time.Now().Add(72 * time.Hour)
	rr := env.AdminPUT(t, "/v1/admin/sessions/"+s2, map[string]any{
		"modality":  "online",
		"starts_at": newStart.Format(time.RFC3339),
		"ends_at":   newStart.Add(time.Hour).Format(time.RFC3339),
	})
	if rr.StatusCode != http.StatusOK {
		t.Fatalf("remarcar status = %d", rr.StatusCode)
	}
	rr.Body.Close()

	// Sessão 3: cancelar com autor.
	s3 := mkSession(96)
	env.AdminPOST(t, "/v1/admin/sessions/"+s3+"/cancel", map[string]any{
		"canceled_by": "terapeuta", "cancel_reason": "agenda cheia",
	}).Body.Close()

	// Histórico.
	hresp := env.AdminGET(t, "/v1/admin/patients/"+pid+"/history")
	if hresp.StatusCode != http.StatusOK {
		t.Fatalf("history status = %d, quer 200", hresp.StatusCode)
	}
	he := DecodeEnvelope(t, hresp)
	var items []struct {
		Tipo   string    `json:"tipo"`
		Data   time.Time `json:"data"`
		Titulo string    `json:"titulo"`
		Resumo string    `json:"resumo"`
		Ref    struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"ref"`
	}
	he.DataAs(t, &items)

	counts := map[string]int{}
	for _, it := range items {
		counts[it.Tipo]++
	}
	for _, tipo := range []string{
		"cadastro", "sessao", "remarcacao", "registro", "nota_livre",
		"debito", "pagamento", "arquivo",
	} {
		if counts[tipo] == 0 {
			t.Errorf("tipo %q ausente no histórico: %+v", tipo, counts)
		}
	}
	// 3 sessões criadas.
	if counts["sessao"] != 3 {
		t.Errorf("sessões = %d, quer 3", counts["sessao"])
	}

	// Ordenação desc.
	for i := 1; i < len(items); i++ {
		if items[i-1].Data.Before(items[i].Data) {
			t.Errorf("ordem não-desc em %d/%d: %v antes de %v", i-1, i, items[i-1].Data, items[i].Data)
		}
	}
}
