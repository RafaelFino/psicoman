package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// portalLogin autentica o paciente via o endpoint nativo do PocketBase
// (auth-with-password da coleção patient_user) e devolve o token de sessão.
func portalLogin(t *testing.T, base, email, password string) string {
	t.Helper()
	body := `{"identity":"` + email + `","password":"` + password + `"}`
	req, _ := http.NewRequest(http.MethodPost,
		base+"/api/collections/patient_user/auth-with-password",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("login do paciente: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login do paciente: status %d (%s)", resp.StatusCode, raw)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		t.Fatalf("token do paciente não obtido: %v (%s)", err, raw)
	}
	return out.Token
}

// portalGet faz um GET no portal com a sessão do paciente (Bearer).
func portalGet(t *testing.T, base, path, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestPortalCadastroLoginEGate (Fase 9): fluxo real de cadastro + login +
// gate de aprovação com a auth REAL da sessão do PocketBase (não mais o seam
// X-Portal-Email). Cobre o caminho negado (pendente → 403) e o permitido
// (aprovado → 2xx) pela mesma sessão.
func TestPortalCadastroLoginEGate(t *testing.T) {
	base := startServer(t)

	const email, pass = "paciente9@exemplo.com", "senha-forte-123"

	// 1. Cadastro pelo portal (cria patient pendente + credencial patient_user).
	status, body := postJSON(t, base, "/v1/portal/register",
		`{"name":"Paciente Nove","phone":"11999990199","email":"`+email+`","password":"`+pass+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("cadastro no portal: status %d (%s)", status, body)
	}
	pid := dataID(t, body)

	// 2. Login real: token de sessão do paciente.
	token := portalLogin(t, base, email, pass)

	// 3. /me funciona mesmo pendente (fora do gate) — identidade real.
	status, body = portalGet(t, base, "/v1/portal/me", token)
	if status != http.StatusOK {
		t.Fatalf("me pendente: quero 200, veio %d (%s)", status, body)
	}

	// 4. Gate NEGADO: pendente não acessa recurso protegido → 403.
	status, body = portalGet(t, base, "/v1/portal/sessions", token)
	if status != http.StatusForbidden {
		t.Fatalf("pendente em /sessions: quero 403, veio %d (%s)", status, body)
	}

	// 5. Terapeuta aprova (rota do app, header do Pangolin).
	status, body = postJSON(t, base, "/v1/app/patients/"+pid+"/approve", ``)
	if status != http.StatusOK {
		t.Fatalf("aprovar paciente: status %d (%s)", status, body)
	}

	// 6. Gate PERMITIDO: a MESMA sessão agora acessa → 2xx.
	status, body = portalGet(t, base, "/v1/portal/sessions", token)
	if status != http.StatusOK {
		t.Fatalf("aprovado em /sessions: quero 200, veio %d (%s)", status, body)
	}
}

// TestPortalSemSessao401 (Fase 9): rota protegida sem Bearer → 401.
func TestPortalSemSessao401(t *testing.T) {
	base := startServer(t)
	status, _, _ := getRaw(t, base, "/v1/portal/sessions")
	// getRaw manda header do Pangolin, mas o portal não o usa; sem Bearer a
	// sessão é inexistente → 401.
	if status != http.StatusUnauthorized {
		t.Fatalf("sem sessão: quero 401, veio %d", status)
	}
}

// registerRaw faz POST /v1/portal/register e devolve status, header Retry-After e corpo.
func registerRaw(t *testing.T, base, email string) (int, string, string) {
	t.Helper()
	body := `{"name":"Limite","phone":"11999990000","email":"` + email + `","password":"senha-forte-123"}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/portal/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST register: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Retry-After"), string(raw)
}

// TestPortalRegisterRateLimit429 (R7.1): 5 cadastros/h por e-mail normalizado.
// O 6º recebe 429 com envelope e Retry-After; outro e-mail segue normal
// (permitido E negado no mesmo teste); o corpo do 429 não ecoa o e-mail.
func TestPortalRegisterRateLimit429(t *testing.T) {
	base := startServer(t)

	// 5 tentativas com o MESMO e-mail variando caixa e espaços: a normalização
	// faz todas caírem na mesma chave. A 1ª cria (201); as demais conflitam
	// (409) — importa que NENHUMA seja 429 até o limite.
	variantes := []string{
		"limite@exemplo.com", "Limite@Exemplo.com", " limite@exemplo.com",
		"LIMITE@EXEMPLO.COM", "limite@exemplo.com ",
	}
	for i, em := range variantes {
		st, _, body := registerRaw(t, base, em)
		if st == http.StatusTooManyRequests {
			t.Fatalf("tentativa %d (dentro do limite) não pode ser 429: %s", i+1, body)
		}
	}

	// 6ª tentativa: 429.
	st, retry, body := registerRaw(t, base, "Limite@exemplo.com")
	if st != http.StatusTooManyRequests {
		t.Fatalf("6ª tentativa: quero 429, veio %d (%s)", st, body)
	}
	if n, err := strconv.Atoi(retry); err != nil || n < 1 {
		t.Errorf("Retry-After deveria ser segundos >= 1, veio %q", retry)
	}
	if !strings.Contains(body, `"code":"limite_excedido"`) {
		t.Errorf("envelope sem code limite_excedido: %s", body)
	}
	if strings.Contains(strings.ToLower(body), "limite@exemplo.com") {
		t.Errorf("o 429 não pode ecoar o e-mail: %s", body)
	}

	// PERMITIDO: outro e-mail segue o fluxo normal (cria 201).
	if st, _, body := registerRaw(t, base, "outro@exemplo.com"); st != http.StatusCreated {
		t.Errorf("outro e-mail deveria cadastrar (201), veio %d (%s)", st, body)
	}
}
