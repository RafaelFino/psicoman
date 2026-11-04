package e2e

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestBootFailClosedSemCifra prova a invariante de segurança fail-closed: sem a
// config de cifra (nem env nem arquivo), o binário NÃO deve servir o contrato
// /v1 — o boot aborta em buildServices. Verifica que /v1/livez não responde num
// prazo generoso (processo não serve).
func TestBootFailClosedSemCifra(t *testing.T) {
	bin := buildBinary(t)
	addr := freePort(t)
	dataDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", dataDir)
	// Ambiente SEM as chaves de cifra: remove-as explicitamente do os.Environ.
	cmd.Env = stripCipherEnv(os.Environ())
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciar binário: %v", err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()

	url := "http://" + addr + "/v1/livez"
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			body := make([]byte, 64)
			resp.Body.Read(body)
			resp.Body.Close()
			t.Fatalf("VIOLAÇÃO fail-closed: /v1/livez respondeu %d sem config de cifra", resp.StatusCode)
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Nenhuma resposta no prazo: comportamento esperado (boot abortou).
}

// TestBootComCifraSobe é o espelho positivo: com a cifra provida por env, o
// binário sobe e /v1/readyz fica pronto (cifra + DB ok; Google degraded).
func TestBootComCifraSobe(t *testing.T) {
	base := startServer(t) // startServer já injeta PSICOMAN_CIPHER_* e aguarda o boot
	status, body := getPublic(t, base, "/v1/readyz")
	if status != http.StatusOK {
		t.Fatalf("com cifra, /readyz deveria responder 200, veio %d (%s)", status, body)
	}
}

// stripCipherEnv devolve o ambiente sem as variáveis de cifra do Psicoman.
func stripCipherEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		if len(kv) >= 16 && kv[:16] == "PSICOMAN_CIPHER_" {
			continue
		}
		out = append(out, kv)
	}
	return out
}
