package e2e

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// startServerWithMetrics sobe o binário real com o listener DEDICADO de métricas
// (PSICOMAN_METRICS_ADDR em uma porta extra) e devolve a base da API e a base do
// listener de métricas. Nenhuma chamada ao Google/Gemini.
func startServerWithMetrics(t *testing.T) (base, metricsBase string) {
	t.Helper()
	bin := buildBinary(t)
	addr := freePort(t)
	maddr := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", t.TempDir())
	cmd.Env = append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
		"PSICOMAN_METRICS_ADDR="+maddr,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	base = "http://" + addr
	metricsBase = "http://" + maddr
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(base + "/v1/health"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base, metricsBase
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("servidor não subiu no prazo")
	return "", ""
}

// httpGet faz GET sem cabeçalhos extras e devolve status, Content-Type e corpo.
func httpGet(t *testing.T, url string) (int, string, string) {
	t.Helper()
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}

func scrapeMetrics(t *testing.T, metricsBase string) string {
	t.Helper()
	st, ct, body := httpGet(t, metricsBase+"/metrics")
	if st != http.StatusOK {
		t.Fatalf("GET /metrics: quero 200, veio %d", st)
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type do /metrics: quero text/plain (formato Prometheus), veio %q", ct)
	}
	return body
}

// TestMetricsExposicaoEIsolamento (R8.1, R8.9.3): o listener dedicado serve o
// formato Prometheus; a API NÃO serve /metrics; o listener NÃO serve a API.
func TestMetricsExposicaoEIsolamento(t *testing.T) {
	base, metricsBase := startServerWithMetrics(t)

	out := scrapeMetrics(t, metricsBase)
	for _, want := range []string{"go_goroutines", "psicoman_build_info", "psicoman_identidade_dev_ativa 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics sem %q", want)
		}
	}

	// A API (porta de negócio) não expõe métricas.
	for _, p := range []string{"/metrics", "/v1/metrics"} {
		if st, _, _ := httpGet(t, base+p); st != http.StatusNotFound {
			t.Errorf("API %s: quero 404, veio %d", p, st)
		}
	}
	// O listener de métricas não serve a API.
	for _, p := range []string{"/v1/livez", "/v1/health", "/"} {
		if st, _, _ := httpGet(t, metricsBase+p); st != http.StatusNotFound {
			t.Errorf("listener de métricas %s: quero 404, veio %d", p, st)
		}
	}
}

// TestMetricsHTTPEMotivos: tráfego real gera as séries com `rota` = padrão
// ({id}), 404 de rota inexistente = nao_encontrada, e 401/403 com o motivo.
// Cobre permitido (2xx) e negado (401/403) juntos.
func TestMetricsHTTPEMotivos(t *testing.T) {
	base, metricsBase := startServerWithMetrics(t)

	httpGet(t, base+"/v1/livez")                                   // 200
	httpGet(t, base+"/v1/app/patients")                            // 401: sem header
	httpGet(t, base+"/v1/portal/sessions")                         // 401: sem sessão
	httpGet(t, base+"/v1/nao-existe")                              // 404: catch-all
	getRaw(t, base, "/v1/app/patients/01HZXAMPLEULID0000000000AB") // 404 de paciente (com header)

	// 403 do portal: cadastro real, login real, paciente NÃO aprovado.
	const email, pass = "metricas403@exemplo.com", "senha-forte-123"
	if st, body := postJSON(t, base, "/v1/portal/register",
		`{"name":"Pendente","phone":"11999990399","email":"`+email+`","password":"`+pass+`"}`); st != http.StatusCreated {
		t.Fatalf("cadastro: %d (%s)", st, body)
	}
	token := portalLogin(t, base, email, pass)
	if st, _ := portalGet(t, base, "/v1/portal/appointment-requests", token); st != http.StatusForbidden {
		t.Fatalf("pendente: quero 403, veio %d", st)
	}

	out := scrapeMetrics(t, metricsBase)
	for _, want := range []string{
		`psicoman_http_requisicoes_total{ator="infra",metodo="GET",rota="/v1/livez",status="200"} 1`,
		`psicoman_http_requisicoes_total{ator="app",metodo="GET",rota="/v1/app/patients",status="401"} 1`,
		`psicoman_http_requisicoes_total{ator="infra",metodo="GET",rota="nao_encontrada",status="404"} 1`,
		`psicoman_http_requisicoes_total{ator="app",metodo="GET",rota="/v1/app/patients/{id}",status="404"} 1`,
		`psicoman_seguranca_negados_total{ator="app",motivo="sem_identidade",status="401"} 1`,
		`psicoman_seguranca_negados_total{ator="portal",motivo="sem_sessao",status="401"} 1`,
		`psicoman_seguranca_negados_total{ator="portal",motivo="nao_aprovado",status="403"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics sem %q", want)
		}
	}
}

var labelRe = regexp.MustCompile(`(\w+)="([^"]*)"`)

// TestMetricsNaoVazaDados (R8.9): sentinelas de dados do paciente, o id real, o
// valor do header de identidade e fragmento da chave de cifra NUNCA aparecem em
// /metrics; os valores dos rótulos ator/metodo/status ficam nos enums.
func TestMetricsNaoVazaDados(t *testing.T) {
	base, metricsBase := startServerWithMetrics(t)

	const (
		nome     = "SENTINELA-NOME-QWV"
		email    = "sentinela-qwv@exemplo.com"
		telefone = "11977665544"
		cpf      = "52998224725"
		nota     = "SENTINELA-NOTA-CLINICA-QWV"
	)
	st, body := postJSON(t, base, "/v1/app/patients",
		`{"name":"`+nome+`","phone":"`+telefone+`","email":"`+email+`","cpf":"`+cpf+`"}`)
	if st != http.StatusCreated {
		t.Fatalf("criar paciente: %d (%s)", st, body)
	}
	pid := dataID(t, body)
	postJSON(t, base, "/v1/app/patients/"+pid+"/notes", `{"content":"`+nota+`"}`)
	getRaw(t, base, "/v1/app/patients/"+pid) // rota com id real

	out := scrapeMetrics(t, metricsBase)
	for _, proibido := range []string{
		nome, email, telefone, cpf, nota, pid,
		"terapeuta@exemplo.com",          // valor de X-Pangolin-User
		"wPb6Yex4abAI5VpFY6SIonnjtPOmtu", // fragmento da chave de cifra
	} {
		if strings.Contains(out, proibido) {
			t.Errorf("VAZAMENTO em /metrics: %q", proibido)
		}
	}

	// Rótulos restritos aos enums fechados.
	atores := map[string]bool{"app": true, "portal": true, "admin": true, "infra": true}
	metodos := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true, "OUTRO": true}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "psicoman_http_requisicoes_total{") {
			continue
		}
		for _, m := range labelRe.FindAllStringSubmatch(line, -1) {
			switch m[1] {
			case "ator":
				if !atores[m[2]] {
					t.Errorf("ator fora do enum: %q", m[2])
				}
			case "metodo":
				if !metodos[m[2]] {
					t.Errorf("metodo fora do enum: %q", m[2])
				}
			case "status":
				if !regexp.MustCompile(`^[1-5]\d\d$`).MatchString(m[2]) {
					t.Errorf("status não numérico: %q", m[2])
				}
			}
		}
	}
}

// TestMetricsFailClosed: sem PSICOMAN_METRICS_ADDR nenhum listener extra abre.
// Reserva uma porta, a FECHA, sobe o servidor SEM a env e confirma que ninguém
// escuta nela. Limitação aceita: outro processo poderia reutilizar a porta no
// intervalo (flake improvável); a garantia primária é o teste unitário que prova
// que, com Addr vazio, nenhum net.Listen é chamado.
func TestMetricsFailClosed(t *testing.T) {
	reservada := freePort(t) // reserva e fecha

	bin := buildBinary(t)
	addr := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--http", addr, "--dir", t.TempDir())
	cmd.Env = append(os.Environ(),
		"PSICOMAN_CIPHER_KEY_ID=k1",
		"PSICOMAN_CIPHER_KEYS=k1:wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY=",
		"PSICOMAN_METRICS_ADDR=", // explicitamente vazio, mesmo que o ambiente traga uma
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("iniciando o binário: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	deadline := time.Now().Add(15 * time.Second)
	up := false
	for time.Now().Before(deadline) {
		if resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(base + "/v1/health"); err == nil {
			_ = resp.Body.Close()
			up = resp.StatusCode == http.StatusOK
			if up {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !up {
		t.Fatal("servidor não subiu no prazo")
	}

	if conn, err := net.DialTimeout("tcp", reservada, time.Second); err == nil {
		_ = conn.Close()
		t.Errorf("sem metrics.addr ninguém deveria escutar em %s", reservada)
	}
	// E a API segue sem /metrics.
	if st, _, _ := httpGet(t, base+"/metrics"); st != http.StatusNotFound {
		t.Errorf("API /metrics: quero 404, veio %d", st)
	}
}
