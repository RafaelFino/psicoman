package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// putJSON faz um PUT autenticado (header do Pangolin) com corpo JSON.
func putJSON(t *testing.T, base, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, base+path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pangolin-User", "terapeuta@exemplo.com")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// deleteReq faz um DELETE autenticado (header do Pangolin).
func deleteReq(t *testing.T, base, path string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, base+path, nil)
	req.Header.Set("X-Pangolin-User", "terapeuta@exemplo.com")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestTherapistProfileContrato (R3.1-R3.2): cria o perfil (PUT), lê (GET),
// adiciona e remove um link de plataforma. O perfil é singleton — um 2º PUT
// atualiza, não duplica.
func TestTherapistProfileContrato(t *testing.T) {
	base := startServer(t)

	// Sem perfil ainda: GET → 404.
	status, _ := getJSONApp(t, base, "/v1/app/therapist")
	if status != http.StatusNotFound {
		t.Fatalf("GET sem perfil: quero 404, veio %d", status)
	}

	// Cria o perfil.
	status, body := putJSON(t, base, "/v1/app/therapist",
		`{"name":"Dra. Ana","crp":"06/12345","email":"ana@exemplo.com","bio":"Atende adultos.","contacts":{"telefone":"5511999990000"},"location_ids":["loc1"]}`)
	if status != http.StatusOK {
		t.Fatalf("criar perfil: status %d (%s)", status, body)
	}
	id1 := dataID(t, body)

	// Lê o perfil.
	status, body = getJSONApp(t, base, "/v1/app/therapist")
	if status != http.StatusOK {
		t.Fatalf("ler perfil: status %d (%s)", status, body)
	}
	var env struct {
		Data struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			CRP   string `json:"crp"`
			Links []struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"links"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decodificar perfil: %v (%s)", err, body)
	}
	if env.Data.Name != "Dra. Ana" || env.Data.CRP != "06/12345" {
		t.Errorf("campos do perfil incorretos: %+v", env.Data)
	}

	// 2º PUT atualiza (singleton): mesmo id, nome novo.
	status, body = putJSON(t, base, "/v1/app/therapist", `{"name":"Dra. Ana Paula","crp":"06/12345"}`)
	if status != http.StatusOK {
		t.Fatalf("atualizar perfil: status %d (%s)", status, body)
	}
	if id2 := dataID(t, body); id2 != id1 {
		t.Errorf("2º PUT criou novo perfil: id1=%s id2=%s", id1, id2)
	}

	// Adiciona um link de plataforma.
	status, body = postJSON(t, base, "/v1/app/therapist/platform-links",
		`{"label":"Doctoralia","url":"https://doctoralia/ana"}`)
	if status != http.StatusCreated {
		t.Fatalf("adicionar link: status %d (%s)", status, body)
	}
	linkID := dataID(t, body)

	// O link aparece no GET do perfil.
	_, body = getJSONApp(t, base, "/v1/app/therapist")
	_ = json.Unmarshal([]byte(body), &env)
	if len(env.Data.Links) != 1 || env.Data.Links[0].ID != linkID {
		t.Fatalf("link não listado no perfil: %+v", env.Data.Links)
	}

	// Remove o link.
	status, _ = deleteReq(t, base, "/v1/app/therapist/platform-links/"+linkID)
	if status != http.StatusOK {
		t.Fatalf("remover link: status %d", status)
	}
	_, body = getJSONApp(t, base, "/v1/app/therapist")
	_ = json.Unmarshal([]byte(body), &env)
	if len(env.Data.Links) != 0 {
		t.Errorf("link não removido: %+v", env.Data.Links)
	}
}

// TestTherapistProfileValida rejeita perfil sem nome (422 do envelope de erro).
func TestTherapistProfileValida(t *testing.T) {
	base := startServer(t)
	status, body := putJSON(t, base, "/v1/app/therapist", `{"name":"   "}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("perfil sem nome: quero 422, veio %d (%s)", status, body)
	}
}

// TestInvoiceComCabecalhoTerapeuta (R3.3): com o perfil configurado, o PDF de
// cobrança passa a imprimir o cabeçalho (nome/CRP). Verifica que os bytes do
// PDF contêm o nome e o CRP do terapeuta.
func TestInvoiceComCabecalhoTerapeuta(t *testing.T) {
	base, suToken := startServerWithSuperuser(t)

	// Configura o perfil do terapeuta.
	status, body := putJSON(t, base, "/v1/app/therapist", `{"name":"Dra. Ana Cabecalho","crp":"06/98765"}`)
	if status != http.StatusOK {
		t.Fatalf("configurar perfil: status %d (%s)", status, body)
	}

	// Paciente + débito semeado.
	status, body = postJSON(t, base, "/v1/app/patients",
		`{"name":"Cobrança Cabeçalho","phone":"11999994322","email":"cobcab@exemplo.com","cpf":"39053344705"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)
	did := seedDebtViaREST(t, base, suToken, pid)

	// Gera o PDF e baixa os bytes.
	status, ibody := postJSON(t, base, "/v1/app/debts/"+did+"/invoice", ``)
	if status != http.StatusCreated {
		t.Fatalf("gerar cobrança: status %d (%s)", status, ibody)
	}
	status, pdfBytes, ctype := getRaw(t, base, "/v1/app/debts/"+did+"/invoice.pdf")
	if status != http.StatusOK || ctype != "application/pdf" {
		t.Fatalf("baixar PDF: status %d ctype %q", status, ctype)
	}
	if !bytes.Contains(pdfBytes, []byte("Dra. Ana Cabecalho")) {
		t.Error("PDF não contém o nome do terapeuta no cabeçalho")
	}
	if !bytes.Contains(pdfBytes, []byte("CRP: 06/98765")) {
		t.Error("PDF não contém o CRP do terapeuta no cabeçalho")
	}
}

// TestPortalTherapistSoPublico (R3.4): o portal expõe só os dados públicos do
// perfil (nome/foto/bio). CRP/email/contatos NUNCA aparecem. Acessível sem
// aprovação (fora do gate), mas exige sessão.
func TestPortalTherapistSoPublico(t *testing.T) {
	base := startServer(t)

	// Perfil completo (com CRP/email/contatos sensíveis).
	status, body := putJSON(t, base, "/v1/app/therapist",
		`{"name":"Dra. Publica","crp":"06/55555","email":"privado@exemplo.com","bio":"Bio pública.","contacts":{"telefone":"5511888887777"}}`)
	if status != http.StatusOK {
		t.Fatalf("configurar perfil: status %d (%s)", status, body)
	}

	// Paciente cadastrado + logado (pendente — o perfil público é fora do gate).
	const email, pass = "portaltp@exemplo.com", "senha-forte-123"
	status, body = postJSON(t, base, "/v1/portal/register",
		`{"name":"Portal TP","phone":"11999990123","email":"`+email+`","password":"`+pass+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("cadastro portal: status %d (%s)", status, body)
	}
	token := portalLogin(t, base, email, pass)

	// Sem sessão → 401 (o perfil não é exposto a anônimos).
	if st, _, _ := getRaw(t, base, "/v1/portal/therapist"); st != http.StatusUnauthorized {
		t.Errorf("portal therapist sem sessão: quero 401, veio %d", st)
	}

	status, pbody := portalGet(t, base, "/v1/portal/therapist", token)
	if status != http.StatusOK {
		t.Fatalf("portal therapist: quero 200, veio %d (%s)", status, pbody)
	}
	// NÃO-VAZAMENTO: o corpo público não contém CRP, email ou telefone.
	for _, sensivel := range []string{"06/55555", "privado@exemplo.com", "5511888887777"} {
		if bytes.Contains([]byte(pbody), []byte(sensivel)) {
			t.Errorf("VAZAMENTO: o perfil público expõe dado sensível %q: %s", sensivel, pbody)
		}
	}
	// Mas contém os dados públicos.
	if !bytes.Contains([]byte(pbody), []byte("Dra. Publica")) || !bytes.Contains([]byte(pbody), []byte("Bio pública.")) {
		t.Errorf("perfil público não traz nome/bio: %s", pbody)
	}
}

// getJSONApp faz um GET autenticado (header do Pangolin) no ator app e devolve
// status + corpo.
func getJSONApp(t *testing.T, base, path string) (int, string) {
	t.Helper()
	status, raw, _ := getRaw(t, base, path)
	return status, string(raw)
}
