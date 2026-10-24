package e2e

import (
	"net/http"
	"testing"
)

// TestBackupSemGoogleNao500 (Fase 7 / D9): disparar o backup SEM credenciais do
// Google responde 2xx — o hook de DR (drivesync) vira no-op observável e NÃO
// derruba a operação (o backup local é criado; o espelho no Drive é
// best-effort, fora do caminho de request). Nenhum teste toca a API real do
// Google.
func TestBackupSemGoogleNao500(t *testing.T) {
	base := startServer(t)

	status, body := postJSON(t, base, "/v1/admin/backup", ``)
	if status != http.StatusAccepted {
		t.Fatalf("backup sem Google: quero 202, veio %d (%s)", status, body)
	}
}
