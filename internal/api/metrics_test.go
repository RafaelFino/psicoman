package api

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"

	"github.com/RafaelFino/psicoman/internal/config"
)

// metricsHarness sobe o listener de métricas em porta efêmera (seam `listen`) e
// o app em processo. base() devolve a URL do listener de métricas.
type metricsHarness struct {
	t     *testing.T
	addr  string
	app   *tests.TestApp
	c     *apiClient
	calls int
}

// withMetrics configura metrics.addr = 127.0.0.1:0 (porta efêmera), captura o
// endereço real via o seam `listen` e devolve o harness com o app já servindo.
func withMetrics(t *testing.T, mutate func(*config.Config)) *metricsHarness {
	t.Helper()
	h := &metricsHarness{t: t}

	prevCfg := configLoader
	configLoader = func() (config.Config, []config.OriginEvent, error) {
		cfg := config.Config{Metrics: config.MetricsConfig{Addr: "127.0.0.1:0"}}
		if mutate != nil {
			mutate(&cfg)
		}
		return cfg, nil, nil
	}
	t.Cleanup(func() { configLoader = prevCfg })

	prevListen := listen
	listen = func(network, address string) (net.Listener, error) {
		h.calls++
		ln, err := net.Listen(network, address)
		if err == nil {
			h.addr = ln.Addr().String()
			t.Cleanup(func() { _ = ln.Close() })
		}
		return ln, err
	}
	t.Cleanup(func() { listen = prevListen })

	h.app = newTestApp(t)
	h.c = newClient(t, h.app)
	return h
}

func (h *metricsHarness) base() string { return "http://" + h.addr }

// scrape faz GET /metrics no listener dedicado.
func (h *metricsHarness) scrape() string {
	h.t.Helper()
	if h.addr == "" {
		h.t.Fatal("listener de métricas não subiu")
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get(h.base() + "/metrics")
	if err != nil {
		h.t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("GET /metrics: quero 200, veio %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestMetrics_ListenerExpoeFormatoPrometheus(t *testing.T) {
	h := withMetrics(t, nil)
	out := h.scrape()
	for _, want := range []string{"go_goroutines", "psicoman_build_info", "psicoman_uptime_segundos", "psicoman_identidade_dev_ativa 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics sem %q", want)
		}
	}
}

// TestMetrics_HTTPSegurancaEPatterns (spike 7.1 em runtime): o Pattern do mux
// alimenta o rótulo `rota` (com {id}, nunca o id real), o 404 de rota
// inexistente vira nao_encontrada, e 401/403 saem com o motivo correto.
func TestMetrics_HTTPSegurancaEPatterns(t *testing.T) {
	h := withMetrics(t, nil)
	c := h.c

	c.do(http.MethodGet, "/v1/livez", "", nil)                                            // 200
	c.do(http.MethodGet, "/v1/app/patients", "", nil)                                     // 401 sem identidade
	c.do(http.MethodGet, "/v1/portal/sessions", "", nil)                                  // 401 sem sessão
	c.do(http.MethodGet, "/v1/nao-existe", "", nil)                                       // 404 catch-all
	c.do(http.MethodGet, "/v1/app/patients/01HZXAMPLEULID0000000000AB", "", appHeaders()) // 404 de paciente

	// 403 do portal: paciente cadastrado e NÃO aprovado, com sessão real.
	registerPortal(t, c, "Pendente", "11999990301", "pendente-metricas@exemplo.com")
	hdr := portalAuthHeader(t, h.app, "pendente-metricas@exemplo.com")
	if res := c.do(http.MethodGet, "/v1/portal/appointment-requests", "", hdr); res.status != http.StatusForbidden {
		t.Fatalf("pendente: quero 403, veio %d", res.status)
	}

	out := h.scrape()
	for _, want := range []string{
		`psicoman_http_requisicoes_total{ator="infra",metodo="GET",rota="/v1/livez",status="200"} 1`,
		`psicoman_http_requisicoes_total{ator="app",metodo="GET",rota="/v1/app/patients",status="401"} 1`,
		`psicoman_http_requisicoes_total{ator="portal",metodo="GET",rota="/v1/portal/sessions",status="401"} 1`,
		`psicoman_http_requisicoes_total{ator="infra",metodo="GET",rota="nao_encontrada",status="404"} 1`,
		`psicoman_http_requisicoes_total{ator="app",metodo="GET",rota="/v1/app/patients/{id}",status="404"} 1`,
		`psicoman_seguranca_negados_total{ator="app",motivo="sem_identidade",status="401"} 1`,
		`psicoman_seguranca_negados_total{ator="portal",motivo="sem_sessao",status="401"} 1`,
		`psicoman_seguranca_negados_total{ator="portal",motivo="nao_aprovado",status="403"} 1`,
		// countingWriter: o /livez escreveu corpo => há amostra de tamanho.
		`psicoman_http_resposta_bytes_count{ator="infra",rota="/v1/livez"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics sem %q", want)
		}
	}
	// Nunca o id real no rótulo.
	if strings.Contains(out, "01HZXAMPLEULID") {
		t.Error("o id da URL não pode virar rótulo")
	}
}

// TestMetrics_NaoVazaDados (R8.9): nenhum dado de paciente, id, ator-identidade
// ou segredo aparece em /metrics.
func TestMetrics_NaoVazaDados(t *testing.T) {
	h := withMetrics(t, nil)
	c := h.c

	const (
		nome     = "SENTINELA-NOME-ZQX"
		email    = "sentinela-zqx@exemplo.com"
		telefone = "11988776655"
		cpf      = "52998224725"
		nota     = "SENTINELA-NOTA-CLINICA-ZQX"
	)
	res := c.do(http.MethodPost, "/v1/app/patients",
		`{"name":"`+nome+`","phone":"`+telefone+`","email":"`+email+`","cpf":"`+cpf+`"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("criar paciente: %d (%s)", res.status, res.body)
	}
	pid := extractID(res.body)
	c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"`+nota+`"}`, appHeaders())
	c.do(http.MethodGet, "/v1/app/patients/"+pid, "", appHeaders())

	out := h.scrape()
	for _, proibido := range []string{
		nome, email, telefone, cpf, nota, pid,
		"terapeuta@exemplo.com",          // valor de X-Pangolin-User
		"wPb6Yex4abAI5VpFY6SIonnjtPOmtu", // fragmento da chave de cifra de teste
	} {
		if strings.Contains(out, proibido) {
			t.Errorf("VAZAMENTO em /metrics: %q", proibido)
		}
	}
}

// TestMetrics_Isolamento (R8.9.3): a API não serve /metrics e o listener de
// métricas não serve a API.
func TestMetrics_Isolamento(t *testing.T) {
	h := withMetrics(t, nil)

	for _, p := range []string{"/metrics", "/v1/metrics"} {
		if res := h.c.do(http.MethodGet, p, "", appHeaders()); res.status != http.StatusNotFound {
			t.Errorf("API %s: quero 404, veio %d", p, res.status)
		}
	}
	for _, p := range []string{"/v1/livez", "/v1/app/patients", "/"} {
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Get(h.base() + p)
		if err != nil {
			t.Fatalf("GET listener %s: %v", p, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("listener de métricas %s: quero 404, veio %d", p, resp.StatusCode)
		}
	}
	// Só GET /metrics: POST é recusado pelo mux.
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Post(h.base()+"/metrics", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("POST /metrics não deveria ser 200")
	}
}

func TestMetrics_IdentidadeDevAtivaGauge(t *testing.T) {
	h := withMetrics(t, func(c *config.Config) { c.Pangolin.DevIdentity = "dev@local" })
	out := h.scrape()
	if !strings.Contains(out, "psicoman_identidade_dev_ativa 1") {
		t.Error("identidade_dev_ativa deveria ser 1 com dev_identity")
	}
	if strings.Contains(out, "dev@local") {
		t.Error("o valor da identidade dev não pode aparecer em /metrics")
	}
}

// TestMetrics_SemAddrNaoAbreSocket (fail-closed): sem metrics.addr, `listen`
// nunca é chamado.
func TestMetrics_SemAddrNaoAbreSocket(t *testing.T) {
	calls := 0
	prevListen := listen
	listen = func(string, string) (net.Listener, error) {
		calls++
		return nil, errors.New("não deveria ser chamado")
	}
	t.Cleanup(func() { listen = prevListen })

	prevCfg := configLoader
	configLoader = func() (config.Config, []config.OriginEvent, error) { return config.Config{}, nil, nil }
	t.Cleanup(func() { configLoader = prevCfg })

	app := newTestApp(t)
	_ = newClient(t, app)
	if calls != 0 {
		t.Errorf("sem metrics.addr nenhum socket deve ser aberto, listen chamado %d vez(es)", calls)
	}
}

// TestStartMetricsServer_FalhaDeBindAbortaOBoot: com addr configurado e porta
// indisponível, StartMetricsServer devolve erro (o boot aborta).
func TestStartMetricsServer_FalhaDeBindAbortaOBoot(t *testing.T) {
	prevListen := listen
	listen = func(string, string) (net.Listener, error) { return nil, errors.New("address already in use") }
	t.Cleanup(func() { listen = prevListen })

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)

	err = StartMetricsServer(app, "127.0.0.1:9091", http.NotFoundHandler(), nil)
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:9091") {
		t.Fatalf("falha de bind deveria devolver erro citando o endereço, veio %v", err)
	}
	// Vazio é fail-closed: sem erro e sem tocar listen.
	if err := StartMetricsServer(app, "  ", http.NotFoundHandler(), nil); err != nil {
		t.Errorf("addr vazio não é erro: %v", err)
	}
}
