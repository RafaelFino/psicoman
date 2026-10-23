// Package e2e cobre os fluxos do binário único do Psicoman via HTTP real,
// subindo o processo com um data dir temporário e as integrações Google/Gemini
// mockadas (ver psicoman-testes-e2e.md). Na Fase 0 há apenas o smoke de boot.
package e2e

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// freePort reserva uma porta TCP livre e a devolve (fecha o listener em seguida;
// há uma janela de corrida teórica, aceitável para o smoke local).
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reservando porta: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// buildBinary compila o binário único cmd/psicoman num arquivo temporário e
// devolve o caminho. A saída é um ARQUIVO, nunca um diretório (psicoman-env).
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "psicoman-test")
	out, err := exec.Command("go", "build", "-o", bin, "../../cmd/psicoman").CombinedOutput()
	if err != nil {
		t.Fatalf("go build do binário único falhou: %v\n%s", err, out)
	}
	return bin
}

// TestBootServeHealth sobe o binário único (PocketBase embarcado) num data dir
// temporário e verifica que /v1/health responde 200 — smoke de boot da Fase 0.
func TestBootServeHealth(t *testing.T) {
	bin := buildBinary(t)
	addr := freePort(t)
	dataDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", dataDir)
	// A partir da Fase 4 o boot exige a config de cifra (buildServices aborta sem
	// ela); provê a mesma KeySpec fixa de teste usada em pb_test.go (H2).
	cmd.Env = append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	url := "http://" + addr + "/v1/health"
	client := &http.Client{Timeout: 2 * time.Second}

	// Espera o servidor subir (boot do PocketBase + migrations internas).
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(150 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /v1/health: status %d, corpo %s", resp.StatusCode, body)
		}
		return // boot OK
	}
	t.Fatalf("servidor não respondeu /v1/health no prazo: %v", lastErr)
}
