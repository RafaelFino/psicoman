package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// startServerWithSuperuser sobe o binário com um superuser semeado (via CLI
// `superuser upsert`) ANTES do serve, e devolve a base URL + um token de
// superuser autenticado. Usado por testes que precisam semear dados em
// coleções trancadas (debt, plan) via a REST do PocketBase — essas coleções não
// têm contrato público (D13), então o seed é feito como operador do sistema.
func startServerWithSuperuser(t *testing.T) (base, suToken string) {
	t.Helper()
	bin := buildBinary(t)
	addr := freePort(t)
	dataDir := t.TempDir()
	env := append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
	)

	const suEmail, suPass = "operador@exemplo.com", "operador-super-secreto-123"
	up := exec.Command(bin, "superuser", "upsert", suEmail, suPass, "--dir", dataDir)
	up.Env = env
	if out, err := up.CombinedOutput(); err != nil {
		t.Fatalf("superuser upsert: %v\n%s", err, out)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", dataDir)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	base = "http://" + addr
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	up2 := false
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/v1/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				up2 = true
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !up2 {
		t.Fatal("servidor não subiu no prazo")
	}

	// Autentica o superuser via REST nativa do PocketBase.
	authBody := `{"identity":"` + suEmail + `","password":"` + suPass + `"}`
	req, _ := http.NewRequest(http.MethodPost,
		base+"/api/collections/_superusers/auth-with-password",
		strings.NewReader(authBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("auth superuser: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth superuser: status %d (%s)", resp.StatusCode, raw)
	}
	var auth struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil || auth.Token == "" {
		t.Fatalf("token de superuser não obtido: %v (%s)", err, raw)
	}
	return base, auth.Token
}

// seedDebtViaREST cria um débito diretamente na coleção debt (trancada, só
// superuser) e devolve o id. Semeia o estado mínimo para exercitar o contrato
// de cobrança sem depender de uma rota de plano (ainda não exposta na API).
func seedDebtViaREST(t *testing.T, base, suToken, patientID string) string {
	t.Helper()
	// id é um ULID fornecido pela app (as coleções não autogeram id — ver
	// newBaseCollection); no seed, provemos um ULID fixo válido (26 chars).
	const debtID = "01HZZZZZZZZZZZZZZZZZZZZZZZ"
	body := map[string]any{
		"id":              debtID,
		"patient_id":      patientID,
		"amount":          15000,
		"status":          "aberto",
		"idempotency_key": "e2e-invoice-" + patientID,
		"due_date":        "2026-05-20T00:00:00-03:00",
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost,
		base+"/api/collections/debt/records", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", suToken)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("criar débito (REST): %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("criar débito (REST): status %d (%s)", resp.StatusCode, out)
	}
	var rec struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &rec); err != nil || rec.ID == "" {
		t.Fatalf("id do débito não obtido: %v (%s)", err, out)
	}
	return rec.ID
}

// getRaw faz um GET autenticado (header do Pangolin) e devolve status + bytes.
func getRaw(t *testing.T, base, path string) (int, []byte, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	req.Header.Set("X-Pangolin-User", "terapeuta@exemplo.com")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, resp.Header.Get("Content-Type")
}

// TestInvoicePDF_GeraEBaixaDoGED (FEAT 6.2 / D9b): cria paciente (ator app),
// semeia um débito (via superuser, coleção trancada), gera o PDF de cobrança e
// o baixa do GED. Verifica o metadado (JSON) e os bytes (application/pdf). SEM
// envio por email — o canal (WhatsApp) é da terapeuta, fora da plataforma.
func TestInvoicePDF_GeraEBaixaDoGED(t *testing.T) {
	base, suToken := startServerWithSuperuser(t)

	// 1. Paciente (ator app).
	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"Cobrança E2E","phone":"11999994321","email":"cobranca@exemplo.com","cpf":"39053344705"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	pid := dataID(t, body)

	// 2. Débito semeado (coleção trancada, só superuser).
	did := seedDebtViaREST(t, base, suToken, pid)

	// 3. Gera o PDF de cobrança (metadado no GED).
	status, ibody := postJSON(t, base, "/v1/app/debts/"+did+"/invoice", ``)
	if status != http.StatusCreated {
		t.Fatalf("gerar cobrança: status %d (%s)", status, ibody)
	}
	var inv struct {
		Data struct {
			FileID    string `json:"file_id"`
			MIME      string `json:"mime"`
			SizeBytes int64  `json:"size_bytes"`
			SHA256    string `json:"sha256"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(ibody), &inv); err != nil {
		t.Fatalf("decodificar cobrança: %v (%s)", err, ibody)
	}
	if inv.Data.FileID == "" || inv.Data.MIME != "application/pdf" || inv.Data.SizeBytes == 0 || inv.Data.SHA256 == "" {
		t.Fatalf("metadado da cobrança incompleto: %+v", inv.Data)
	}

	// 4. Baixa os BYTES do PDF do GED.
	status, pdfBytes, ctype := getRaw(t, base, "/v1/app/debts/"+did+"/invoice.pdf")
	if status != http.StatusOK {
		t.Fatalf("baixar PDF: status %d", status)
	}
	if ctype != "application/pdf" {
		t.Errorf("Content-Type esperado application/pdf, veio %q", ctype)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("download não é um PDF válido (prefixo: %q)", pdfBytes[:min(16, len(pdfBytes))])
	}
	if int64(len(pdfBytes)) != inv.Data.SizeBytes {
		t.Errorf("tamanho do download (%d) difere do metadado (%d)", len(pdfBytes), inv.Data.SizeBytes)
	}
}

// dataID extrai data.id de um envelope de resposta.
func dataID(t *testing.T, body string) string {
	t.Helper()
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decodificar envelope: %v (%s)", err, body)
	}
	if env.Data.ID == "" {
		t.Fatalf("id vazio no envelope: %s", body)
	}
	return env.Data.ID
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
