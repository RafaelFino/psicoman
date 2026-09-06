// Package e2e contém a suíte end-to-end. Cada teste sobe uma instância do
// servidor (admin ou portal) com SQLite temporário e integrações Google
// mockadas, exercitando os fluxos via HTTP real (psicoman-testes-e2e.md).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
)

const (
	testAdminEmail  = "terapeuta@example.com"
	testAdminSecret = "e2e-secret"
)

// Env é o ambiente de um teste E2E: servidor rodando + helpers HTTP.
type Env struct {
	BaseURL string
	Google  *google.FakeClient
	cfg     *config.Config
	cancel  context.CancelFunc
	done    chan struct{}
	client  *http.Client
}

// freePort devolve uma porta TCP livre.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("alocando porta: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// testConfig monta um config temporário isolado para o teste.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Admin.Host = "127.0.0.1"
	cfg.Admin.Port = freePort(t)
	cfg.Portal.Host = "127.0.0.1"
	cfg.Portal.Port = freePort(t)
	cfg.Paths.SQLite = filepath.Join(dir, "psicoman.db")
	cfg.Paths.GEDRoot = filepath.Join(dir, "ged")
	cfg.Paths.LogDir = "" // só stdout nos testes
	cfg.AdminAuth.Email = testAdminEmail
	cfg.AdminAuth.Secret = testAdminSecret
	cfg.AdminAuth.EmailHeader = "X-Pangolin-Email"
	cfg.AdminAuth.SecretHeader = "X-Pangolin-Secret"
	cfg.Log.Level = "error"
	cfg.Google.CalendarID = "primary"
	// Chave AES-256 fixa de teste (32 bytes base64) para cifrar tokens/backup.
	cfg.Crypto.Key = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	cfg.Reminders.MinutesBefore = []int{1440, 30}
	cfg.RateLimit.RequestsPerMinute = 1000
	cfg.RateLimit.Burst = 1000
	return cfg
}

// StartAdmin sobe o servidor admin em processo com o config de teste e um
// cliente Google fake (nenhuma rede).
func StartAdmin(t *testing.T) *Env {
	t.Helper()
	return StartAdminWithGoogle(t, google.NewFakeClient())
}

// StartAdminWithGoogle permite injetar um FakeClient específico (ex: com
// intervalos ocupados para testar conflito de agenda).
func StartAdminWithGoogle(t *testing.T, fake *google.FakeClient) *Env {
	t.Helper()
	cfg := testConfig(t)
	env := start(t, cfg, cfg.Admin.Addr(), func(ctx context.Context, c *config.Config) (app.Runnable, error) {
		return app.NewAdminForTest(ctx, c, app.Options{
			Calendar: fake,
			Gmail:    fake,
			Drive:    fake,
		})
	})
	env.Google = fake
	return env
}

// StartPortal sobe o servidor portal em processo com login social fake
// (a credencial de login é tratada como o próprio email).
func StartPortal(t *testing.T) *Env {
	t.Helper()
	cfg := testConfig(t)
	return start(t, cfg, cfg.Portal.Addr(), func(ctx context.Context, c *config.Config) (app.Runnable, error) {
		return app.NewPortalForTest(ctx, c, app.Options{Identity: google.FakeIdentityVerifier{}})
	})
}

// StartAdminAndPortal sobe admin e portal compartilhando o MESMO banco/GED,
// para os fluxos que cruzam as duas superfícies (ex: paciente do portal visto
// no admin). Devolve (admin, portal).
func StartAdminAndPortal(t *testing.T) (*Env, *Env) {
	t.Helper()
	cfg := testConfig(t)
	fake := google.NewFakeClient()
	adminEnv := start(t, cfg, cfg.Admin.Addr(), func(ctx context.Context, c *config.Config) (app.Runnable, error) {
		return app.NewAdminForTest(ctx, c, app.Options{Calendar: fake, Gmail: fake, Drive: fake})
	})
	adminEnv.Google = fake
	portalEnv := start(t, cfg, cfg.Portal.Addr(), func(ctx context.Context, c *config.Config) (app.Runnable, error) {
		return app.NewPortalForTest(ctx, c, app.Options{Identity: google.FakeIdentityVerifier{}})
	})
	return adminEnv, portalEnv
}

func start(t *testing.T, cfg *config.Config, addr string, build func(context.Context, *config.Config) (app.Runnable, error)) *Env {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	runnable, err := build(ctx, cfg)
	if err != nil {
		cancel()
		t.Fatalf("montando servidor de teste: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = runnable.Run(ctx)
		close(done)
	}()

	env := &Env{
		BaseURL: "http://" + addr,
		cfg:     cfg,
		cancel:  cancel,
		done:    done,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	env.waitReady(t)
	return env
}

func (e *Env) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := e.client.Get(e.BaseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("servidor não ficou pronto a tempo")
}

// Stop encerra o servidor.
func (e *Env) Stop() {
	e.cancel()
	select {
	case <-e.done:
	case <-time.After(5 * time.Second):
	}
}

// GET faz uma requisição GET sem autenticação.
func (e *Env) GET(t *testing.T, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.BaseURL+path, nil)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// AdminGET faz GET autenticado como admin.
func (e *Env) AdminGET(t *testing.T, path string) *http.Response {
	return e.adminDo(t, http.MethodGet, path, nil)
}

// AdminPOST faz POST autenticado como admin com corpo JSON.
func (e *Env) AdminPOST(t *testing.T, path string, body any) *http.Response {
	return e.adminDo(t, http.MethodPost, path, body)
}

// AdminPUT faz PUT autenticado como admin com corpo JSON.
func (e *Env) AdminPUT(t *testing.T, path string, body any) *http.Response {
	return e.adminDo(t, http.MethodPut, path, body)
}

// AdminDELETE faz DELETE autenticado como admin.
func (e *Env) AdminDELETE(t *testing.T, path string) *http.Response {
	return e.adminDo(t, http.MethodDelete, path, nil)
}

func (e *Env) adminDo(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.BaseURL+path, r)
	req.Header.Set(e.cfg.AdminAuth.EmailHeader, testAdminEmail)
	req.Header.Set(e.cfg.AdminAuth.SecretHeader, testAdminSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// PortalClient é um cliente HTTP do portal com cookie jar (mantém a sessão).
type PortalClient struct {
	env    *Env
	client *http.Client
}

// NewPortalClient cria um cliente com jar de cookies para o portal.
func (e *Env) NewPortalClient(t *testing.T) *PortalClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &PortalClient{env: e, client: &http.Client{Timeout: 10 * time.Second, Jar: jar}}
}

// POST faz um POST JSON mantendo cookies de sessão.
func (p *PortalClient) POST(t *testing.T, path string, body any) *http.Response {
	return p.do(t, http.MethodPost, path, body)
}

// PUT faz um PUT JSON mantendo cookies de sessão.
func (p *PortalClient) PUT(t *testing.T, path string, body any) *http.Response {
	return p.do(t, http.MethodPut, path, body)
}

// GET faz um GET mantendo cookies de sessão.
func (p *PortalClient) GET(t *testing.T, path string) *http.Response {
	return p.do(t, http.MethodGet, path, nil)
}

func (p *PortalClient) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, p.env.BaseURL+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// DecodeEnvelope lê o envelope padrão da resposta.
func DecodeEnvelope(t *testing.T, resp *http.Response) Envelope {
	t.Helper()
	defer resp.Body.Close()
	var env Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decodificando envelope: %v", err)
	}
	return env
}

// Envelope espelha o contrato de resposta da API.
type Envelope struct {
	Message   string          `json:"message"`
	ElapsedMS int64           `json:"elapsed_ms"`
	Data      json.RawMessage `json:"data"`
	Error     json.RawMessage `json:"error"`
	RequestID string          `json:"request_id"`
}

// DataAs desserializa o campo data em dst.
func (env Envelope) DataAs(t *testing.T, dst any) {
	t.Helper()
	if len(env.Data) == 0 {
		t.Fatal("envelope sem data")
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
}

var _ = os.Getenv
var _ = fmt.Sprintf
