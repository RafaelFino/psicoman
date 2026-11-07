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

// doReq faz uma requisição e devolve status + corpo. Se actor != "", envia o
// header X-Pangolin-User; se "" (anônimo), não envia (exercita o 401).
func doReq(t *testing.T, method, url, actor, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = bytes.NewReader([]byte(body))
	}
	req, _ := http.NewRequest(method, url, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Pangolin-User", actor)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestAuditProntuario cobre a trilha de auditoria clínica (R1/R5/R7.4):
// leitura/escrita do prontuário gravam audit_log com o ator real (header), sem
// vazar texto clínico; acesso sem identidade é negado (401) e não audita.
func TestAuditProntuario(t *testing.T) {
	base := startServer(t)
	const actor = "terapeuta@exemplo.com"
	const notaClinica = "paciente-relata-insonia-e-angustia-confidencial"
	const anamneseClinica = "historico-familiar-detalhe-sensivel"

	// Cria paciente.
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Auditoria E2E","phone":"11999990000","email":"audit-e2e@exemplo.com"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: %d (%s)", status, body)
	}
	var pat struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &pat); err != nil || pat.Data.ID == "" {
		t.Fatalf("decodificar paciente: %v (%s)", err, body)
	}
	pid := pat.Data.ID

	// Escreve nota e anamnese, depois lê o prontuário — todos com identidade.
	if st, b := doReq(t, http.MethodPost, base+"/v1/app/patients/"+pid+"/notes", actor,
		`{"content":"`+notaClinica+`"}`); st != http.StatusCreated && st != http.StatusOK {
		t.Fatalf("POST notes: %d (%s)", st, b)
	}
	if st, b := doReq(t, http.MethodPut, base+"/v1/app/patients/"+pid+"/anamnesis", actor,
		`{"content":"`+anamneseClinica+`"}`); st != http.StatusOK && st != http.StatusCreated {
		t.Fatalf("PUT anamnesis: %d (%s)", st, b)
	}
	if st, b := doReq(t, http.MethodGet, base+"/v1/app/patients/"+pid, actor, ""); st != http.StatusOK {
		t.Fatalf("GET chart: %d (%s)", st, b)
	}

	// Lê a trilha de auditoria.
	st, auditBody := doReq(t, http.MethodGet, base+"/v1/admin/audit-log?limit=200", actor, "")
	if st != http.StatusOK {
		t.Fatalf("GET audit-log: %d (%s)", st, auditBody)
	}

	// 1) O texto clínico NUNCA aparece no JSON inteiro da trilha.
	if strings.Contains(auditBody, notaClinica) || strings.Contains(auditBody, anamneseClinica) ||
		strings.Contains(auditBody, "confidencial") || strings.Contains(auditBody, "sensivel") {
		t.Fatalf("audit-log vazou texto clínico: %s", auditBody)
	}

	// 2) As ações esperadas estão na trilha, com o ator real e as entidades certas.
	var audit struct {
		Data []struct {
			Actor    string `json:"actor"`
			Action   string `json:"action"`
			Entity   string `json:"entity"`
			EntityID string `json:"entity_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(auditBody), &audit); err != nil {
		t.Fatalf("decodificar audit-log: %v (%s)", err, auditBody)
	}
	want := map[string]string{ // entidade -> ação esperada
		"note":       "criar",
		"anamnesis":  "criar",
		"prontuario": "ler",
	}
	seen := map[string]bool{}
	for _, e := range audit.Data {
		if exp, ok := want[e.Entity]; ok && e.Action == exp {
			if e.Actor != actor {
				t.Errorf("ator esperado %q na entrada %s/%s, veio %q", actor, e.Entity, e.Action, e.Actor)
			}
			seen[e.Entity] = true
		}
	}
	for ent := range want {
		if !seen[ent] {
			t.Errorf("faltou entrada de auditoria para a entidade %q (ação %q)", ent, want[ent])
		}
	}

	// 3) NEGADO: GET do prontuário sem identidade → 401 e nenhuma auditoria nova.
	stBefore, _ := doReq(t, http.MethodGet, base+"/v1/admin/audit-log?limit=500", actor, "")
	_ = stBefore
	countBefore := len(audit.Data)
	if st, b := doReq(t, http.MethodGet, base+"/v1/app/patients/"+pid, "", ""); st != http.StatusUnauthorized {
		t.Fatalf("GET chart sem header: quero 401, veio %d (%s)", st, b)
	}
	_, auditBody2 := doReq(t, http.MethodGet, base+"/v1/admin/audit-log?limit=500", actor, "")
	var audit2 struct {
		Data []struct {
			Entity string `json:"entity"`
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(auditBody2), &audit2); err != nil {
		t.Fatalf("decodificar audit-log (2): %v", err)
	}
	// Nenhuma nova leitura de prontuário entrou por conta do 401.
	reads := 0
	for _, e := range audit2.Data {
		if e.Entity == "prontuario" && e.Action == "ler" {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("o 401 não deveria gerar auditoria de leitura; leituras de prontuario na trilha = %d (antes havia %d entradas)", reads, countBefore)
	}
}

// TestAuditSessionCancel confere que o cancelamento de sessão grava o ator real
// no audit_log (session.cancel), via propagação pelo context.
func TestAuditSessionCancel(t *testing.T) {
	base := startServer(t)
	const actor = "terapeuta@exemplo.com"

	// Paciente + sessão agendada.
	_, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Cancel E2E","phone":"11999990001","email":"cancel-e2e@exemplo.com"}`)
	var pat struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &pat); err != nil || pat.Data.ID == "" {
		t.Fatalf("decodificar paciente: %v (%s)", err, body)
	}
	_, sbody := postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pat.Data.ID+`","modality":"online","starts_at":"2026-12-22T14:00:00-03:00","ends_at":"2026-12-22T15:00:00-03:00"}`)
	var sess struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(sbody), &sess); err != nil || sess.Data.ID == "" {
		t.Fatalf("decodificar sessão: %v (%s)", err, sbody)
	}

	if st, b := doReq(t, http.MethodPost, base+"/v1/app/sessions/"+sess.Data.ID+"/cancel", actor,
		`{"reason":"teste"}`); st != http.StatusOK {
		t.Fatalf("cancelar sessão: %d (%s)", st, b)
	}

	_, auditBody := doReq(t, http.MethodGet, base+"/v1/admin/audit-log?limit=200", actor, "")
	var audit struct {
		Data []struct {
			Actor  string `json:"actor"`
			Action string `json:"action"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(auditBody), &audit); err != nil {
		t.Fatalf("decodificar audit-log: %v", err)
	}
	found := false
	for _, e := range audit.Data {
		if e.Action == "session.cancel" {
			found = true
			if e.Actor != actor {
				t.Errorf("session.cancel: ator esperado %q, veio %q", actor, e.Actor)
			}
		}
	}
	if !found {
		t.Errorf("faltou entrada session.cancel no audit-log: %s", auditBody)
	}
}
