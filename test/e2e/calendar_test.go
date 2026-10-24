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

// startServer sobe o binário único sem credenciais do Google (Calendar degrada
// no-op — FEAT-002/D8) e devolve a base URL. Reaproveita buildBinary/freePort.
// NENHUM teste toca a API real do Google.
func startServer(t *testing.T) string {
	t.Helper()
	bin := buildBinary(t)
	addr := freePort(t)
	dataDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", dataDir)
	cmd.Env = append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/v1/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("servidor não subiu no prazo")
	return ""
}

func postJSON(t *testing.T, base, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pangolin-User", "terapeuta@exemplo.com")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestAgendaSemGoogleAgenda201 (FEAT-002): sem credenciais do Google no
// ambiente, agendar uma sessão responde 201 sem evento e SEM panic (degradação
// do Calendar). O cenário de conflito (409) é coberto em processo no pacote
// internal/api, pois o binário real não injeta um fake HTTP do Google.
func TestAgendaSemGoogleAgenda201(t *testing.T) {
	base := startServer(t)

	status, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"E2E Paciente","phone":"11999991234","email":"e2e@exemplo.com"}`)
	if status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", status, body)
	}
	var pat struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &pat); err != nil {
		t.Fatalf("decodificar paciente: %v (%s)", err, body)
	}
	if pat.Data.ID == "" {
		t.Fatalf("id do paciente vazio: %s", body)
	}

	status, body = postJSON(t, base, "/v1/app/sessions",
		`{"patient_id":"`+pat.Data.ID+`","modality":"online","starts_at":"2026-12-21T14:00:00-03:00","ends_at":"2026-12-21T15:00:00-03:00"}`)
	if status != http.StatusCreated {
		t.Fatalf("agendar sessão sem Google: quero 201, veio %d (%s)", status, body)
	}
	// Sem Google, nenhum MeetURL real é preenchido.
	if strings.Contains(body, "meet.google.com") {
		t.Errorf("sem Google não deveria vir MeetURL real: %s", body)
	}
}
