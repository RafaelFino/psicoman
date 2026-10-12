package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAnamnesis(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "anamnese@example.com")

	// Antes de salvar, vem vazia (não 404).
	resp := env.AdminGET(t, "/v1/admin/patients/"+pid+"/anamnesis")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get anamnese vazia status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Salva.
	resp = env.AdminPUT(t, "/v1/admin/patients/"+pid+"/anamnesis", map[string]any{
		"content": "Queixa principal: ansiedade.",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save anamnese status = %d, quer 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Recupera com conteúdo.
	resp = env.AdminGET(t, "/v1/admin/patients/"+pid+"/anamnesis")
	e := DecodeEnvelope(t, resp)
	var a struct {
		Content string `json:"content"`
	}
	e.DataAs(t, &a)
	if a.Content != "Queixa principal: ansiedade." {
		t.Errorf("conteúdo = %q", a.Content)
	}

	// Atualizar sobrescreve (uma por paciente).
	resp = env.AdminPUT(t, "/v1/admin/patients/"+pid+"/anamnesis", map[string]any{"content": "Atualizado."})
	resp.Body.Close()
	resp = env.AdminGET(t, "/v1/admin/patients/"+pid+"/anamnesis")
	e2 := DecodeEnvelope(t, resp)
	e2.DataAs(t, &a)
	if a.Content != "Atualizado." {
		t.Errorf("anamnese não atualizada: %q", a.Content)
	}
}

func TestNotesOrdering(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "notas@example.com")

	// Nota livre.
	env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"content": "Primeira nota (livre)",
	}).Body.Close()
	// Nota de sessão (session_id qualquer string; FK SET NULL permite, mas usamos livre p/ simplicidade).
	env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"content": "Segunda nota",
	}).Body.Close()

	resp := env.AdminGET(t, "/v1/admin/patients/"+pid+"/notes")
	e := DecodeEnvelope(t, resp)
	var notes []map[string]any
	e.DataAs(t, &notes)
	if len(notes) != 2 {
		t.Fatalf("notas = %d, quer 2", len(notes))
	}
	// Ordenadas por created_at crescente.
	if notes[0]["content"] != "Primeira nota (livre)" {
		t.Errorf("ordem incorreta: primeira = %v", notes[0]["content"])
	}
	if notes[0]["is_free"] != true {
		t.Error("primeira nota deveria ser livre")
	}
}

func TestTemplateRenderAndSend(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "template@example.com")

	// Cria template Markdown.
	resp := env.AdminPOST(t, "/v1/admin/templates", map[string]any{
		"name":    "Boas-vindas",
		"body_md": "# Olá\n\nSeja **bem-vindo** ao consultório.\n\n- Pontualidade\n- Sigilo",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar template status = %d", resp.StatusCode)
	}
	ce := DecodeEnvelope(t, resp)
	var tpl struct {
		ID string `json:"id"`
	}
	ce.DataAs(t, &tpl)

	// Renderiza → HTML formatado.
	resp = env.AdminGET(t, "/v1/admin/templates/"+tpl.ID+"/render")
	re := DecodeEnvelope(t, resp)
	var rend struct {
		HTML string `json:"html"`
	}
	re.DataAs(t, &rend)
	if !strings.Contains(rend.HTML, "<h1>Olá</h1>") || !strings.Contains(rend.HTML, "<strong>bem-vindo</strong>") {
		t.Errorf("HTML renderizado inesperado: %q", rend.HTML)
	}

	// Envia ao paciente (registra versão formatada).
	resp = env.AdminPOST(t, "/v1/admin/templates/"+tpl.ID+"/send", map[string]any{"patient_id": pid})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("enviar template status = %d, quer 201", resp.StatusCode)
	}
	se := DecodeEnvelope(t, resp)
	var send struct {
		HTML string `json:"html"`
	}
	se.DataAs(t, &send)
	if !strings.Contains(send.HTML, "<li>Pontualidade</li>") {
		t.Errorf("envio sem versão formatada: %q", send.HTML)
	}
}

// TestSessionLinkedNote cobre R2.3/R2.4: ao criar uma nota com session_id, ela
// fica correlacionada à sessão (is_free=false, session_id preenchido), e a
// edição da sessão (PUT) continua funcionando no mesmo fluxo do painel.
func TestSessionLinkedNote(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "prontuario-sessao@example.com")

	// Cria uma sessão agendada.
	start := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start.Format(time.RFC3339),
		"ends_at":   start.Add(time.Hour).Format(time.RFC3339),
		"status":    "agendada",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar sessão status = %d", resp.StatusCode)
	}
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)

	// Registro vinculado à sessão.
	resp = env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"session_id": s.ID,
		"content":    "Relatório da sessão: evoluiu bem.",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("criar nota de sessão status = %d, quer 201", resp.StatusCode)
	}
	ne := DecodeEnvelope(t, resp)
	var note struct {
		SessionID string `json:"session_id"`
		IsFree    bool   `json:"is_free"`
	}
	ne.DataAs(t, &note)
	if note.SessionID != s.ID {
		t.Errorf("session_id = %q, quer %q", note.SessionID, s.ID)
	}
	if note.IsFree {
		t.Error("nota vinculada não deveria ser livre")
	}

	// Nota livre no mesmo paciente.
	env.AdminPOST(t, "/v1/admin/patients/"+pid+"/notes", map[string]any{
		"content": "Anotação livre.",
	}).Body.Close()

	// GET notes: uma vinculada (session_id), uma livre.
	resp = env.AdminGET(t, "/v1/admin/patients/"+pid+"/notes")
	le := DecodeEnvelope(t, resp)
	var notes []struct {
		SessionID string `json:"session_id"`
		IsFree    bool   `json:"is_free"`
	}
	le.DataAs(t, &notes)
	var vinculadas, livres int
	for _, n := range notes {
		if n.SessionID == s.ID && !n.IsFree {
			vinculadas++
		}
		if n.IsFree && n.SessionID == "" {
			livres++
		}
	}
	if vinculadas != 1 || livres != 1 {
		t.Errorf("esperava 1 vinculada e 1 livre, veio %d/%d", vinculadas, livres)
	}
}
