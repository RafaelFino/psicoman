package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// fakeRecorder registra o que o middleware reporta.
type fakeRecorder struct {
	mu       sync.Mutex
	requests []reqRec
	inflight map[string]int
	denied   []string // "ator:status:motivo"
}

type reqRec struct {
	ator, rota, metodo string
	status             int
	bytes              int64
}

func newFakeRecorder() *fakeRecorder { return &fakeRecorder{inflight: map[string]int{}} }

func (f *fakeRecorder) Request(ator, rota, metodo string, status int, _ time.Duration, bytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, reqRec{ator, rota, metodo, status, bytes})
}
func (f *fakeRecorder) InFlight(ator string, d int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inflight[ator] += d
}
func (f *fakeRecorder) Denied(ator string, status int, motivo string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.denied = append(f.denied, ator+":"+strings.TrimSpace(http.StatusText(status))+":"+motivo)
}
func (f *fakeRecorder) RateLimited(string) {}

// panicRecorder panica em todo método: a resposta não pode ser afetada (R8.10).
type panicRecorder struct{}

func (panicRecorder) Request(string, string, string, int, time.Duration, int64) { panic("boom") }
func (panicRecorder) InFlight(string, int)                                      { panic("boom") }
func (panicRecorder) Denied(string, int, string)                                { panic("boom") }
func (panicRecorder) RateLimited(string)                                        { panic("boom") }

// newTrackedEvent devolve um RequestEvent cujo Response é o ResponseWriter do
// PocketBase (rastreia status/escrita), como no servidor real.
func newTrackedEvent(t *testing.T, method, path, pattern string) (*core.RequestEvent, *httptest.ResponseRecorder) {
	t.Helper()
	e, rec := newEvent(t)
	e.Request = httptest.NewRequest(method, path, nil)
	e.Request.Pattern = pattern
	e.Response = &router.ResponseWriter{ResponseWriter: rec}
	return e, rec
}

func TestRouteLabels(t *testing.T) {
	casos := []struct{ pattern, ator, rota string }{
		{"GET /v1/app/patients/{id}", "app", "/v1/app/patients/{id}"},
		{"POST /v1/app/patients/{id}/notes", "app", "/v1/app/patients/{id}/notes"},
		{"GET /v1/portal/sessions", "portal", "/v1/portal/sessions"},
		{"POST /v1/portal/register", "portal", "/v1/portal/register"},
		{"GET /v1/admin/audit-log", "admin", "/v1/admin/audit-log"},
		{"GET /v1/health", "infra", "/v1/health"},
		{"GET /v1/livez", "infra", "/v1/livez"},
		{"GET /api/health", "infra", "/api/health"},
		{"/v1/app/legado", "app", "/v1/app/legado"}, // sem prefixo de método
		{"/", "infra", "nao_encontrada"},            // catch-all do PocketBase
		{"", "infra", "desconhecida"},
		{"   ", "infra", "desconhecida"},
	}
	for _, c := range casos {
		ator, rota := RouteLabels(c.pattern)
		if ator != c.ator || rota != c.rota {
			t.Errorf("RouteLabels(%q) = (%q, %q); quero (%q, %q)", c.pattern, ator, rota, c.ator, c.rota)
		}
	}
}

// TestRouteLabels_NuncaVazaIdentificador: o rótulo vem do padrão ({id}), nunca
// do caminho com o id real — um ULID jamais vira rótulo.
func TestRouteLabels_NuncaVazaIdentificador(t *testing.T) {
	const ulid = "01HZXAMPLEULID0000000000AB"
	_, rota := RouteLabels("GET /v1/app/patients/{id}")
	if strings.Contains(rota, ulid) || !strings.Contains(rota, "{id}") {
		t.Errorf("rota deveria usar o padrão {id}: %q", rota)
	}
}

func TestMetodoLabel(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		if metodoLabel(m) != m {
			t.Errorf("%s deveria ser mantido", m)
		}
	}
	if metodoLabel("BREW") != "OUTRO" || metodoLabel("x"+strings.Repeat("a", 500)) != "OUTRO" {
		t.Error("método fora do conjunto deve virar OUTRO (cardinalidade fechada)")
	}
}

func TestInstrument_ContaRequisicaoEBytes(t *testing.T) {
	rec := newFakeRecorder()
	e, _ := newTrackedEvent(t, http.MethodGet, "/v1/app/patients/01HZXAMPLE", "GET /v1/app/patients/{id}")

	if err := Instrument(rec)(e); err != nil {
		t.Fatalf("Instrument: %v", err)
	}
	if len(rec.requests) != 1 {
		t.Fatalf("quero 1 requisição registrada, veio %d", len(rec.requests))
	}
	got := rec.requests[0]
	if got.ator != "app" || got.rota != "/v1/app/patients/{id}" || got.metodo != "GET" || got.status != 200 {
		t.Errorf("registro inesperado: %+v", got)
	}
	if got.bytes != 0 {
		t.Errorf("nada foi escrito: bytes deveria ser 0, veio %d", got.bytes)
	}
	// O gauge volta a zero (+1 antes, -1 depois) e usa o ator do caminho.
	if rec.inflight["app"] != 0 {
		t.Errorf("em andamento deveria voltar a 0, veio %d", rec.inflight["app"])
	}
	// O Response original é restaurado após a cadeia.
	if _, ok := e.Response.(*countingWriter); ok {
		t.Error("e.Response deveria ser restaurado ao writer original")
	}
}

// TestInstrument_NegacaoComMotivo: 401 gravado com SetDenyReason é contado com o
// motivo; sem motivo cai em "outro" (ou "pocketbase" para /api/*).
func TestInstrument_NegacaoComMotivo(t *testing.T) {
	rec := newFakeRecorder()
	e, _ := newTrackedEvent(t, http.MethodGet, "/v1/app/patients", "GET /v1/app/patients")
	SetDenyReason(e, DenySemIdentidade)
	e.Response.WriteHeader(http.StatusUnauthorized)

	if err := Instrument(rec)(e); err != nil {
		t.Fatal(err)
	}
	if rec.requests[0].status != 401 || len(rec.denied) != 1 || !strings.HasSuffix(rec.denied[0], ":sem_identidade") {
		t.Errorf("negação inesperada: req=%+v denied=%v", rec.requests, rec.denied)
	}

	// 403 sem motivo em rota comum => "outro".
	rec2 := newFakeRecorder()
	e2, _ := newTrackedEvent(t, http.MethodGet, "/v1/portal/x", "GET /v1/portal/x")
	e2.Response.WriteHeader(http.StatusForbidden)
	_ = Instrument(rec2)(e2)
	if len(rec2.denied) != 1 || !strings.HasSuffix(rec2.denied[0], ":outro") {
		t.Errorf("403 sem motivo deveria ser 'outro': %v", rec2.denied)
	}

	// 401 em /api/* sem motivo => "pocketbase".
	rec3 := newFakeRecorder()
	e3, _ := newTrackedEvent(t, http.MethodGet, "/api/collections", "GET /api/collections")
	e3.Response.WriteHeader(http.StatusUnauthorized)
	_ = Instrument(rec3)(e3)
	if len(rec3.denied) != 1 || !strings.HasSuffix(rec3.denied[0], ":pocketbase") {
		t.Errorf("401 em /api/ sem motivo deveria ser 'pocketbase': %v", rec3.denied)
	}
}

func TestInstrument_SemNegacaoNaoChamaDenied(t *testing.T) {
	rec := newFakeRecorder()
	e, _ := newTrackedEvent(t, http.MethodGet, "/v1/livez", "GET /v1/livez")
	e.Response.WriteHeader(http.StatusOK)
	_ = Instrument(rec)(e)
	if len(rec.denied) != 0 {
		t.Errorf("200 não deveria gerar negação: %v", rec.denied)
	}
}

// TestInstrument_RecorderQuePanicaNaoAlteraAResposta (R8.10).
func TestInstrument_RecorderQuePanicaNaoAlteraAResposta(t *testing.T) {
	e, rec := newTrackedEvent(t, http.MethodGet, "/v1/livez", "GET /v1/livez")
	e.Response.WriteHeader(http.StatusTeapot)
	if err := Instrument(panicRecorder{})(e); err != nil {
		t.Fatalf("recorder que panica não pode virar erro: %v", err)
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("a resposta não pode ser alterada: quero 418, veio %d", rec.Code)
	}
}

// TestCountingWriter_ConservaTrackersEFlush (spike 7.1): o wrapper conta bytes e,
// via Unwrap(), mantém Status()/Written() reconhecidos pelo PocketBase e o
// Flush (SSE de /api/realtime) funcionando por http.ResponseController.
func TestCountingWriter_ConservaTrackersEFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	orig := &router.ResponseWriter{ResponseWriter: rec}
	cw := &countingWriter{ResponseWriter: orig}

	ev := &router.Event{Response: cw}
	cw.WriteHeader(http.StatusCreated)
	if n, err := cw.Write([]byte("olá")); err != nil || n != len("olá") {
		t.Fatalf("Write: n=%d err=%v", n, err)
	}
	if cw.n != int64(len("olá")) {
		t.Errorf("bytes contados: quero %d, veio %d", len("olá"), cw.n)
	}
	if ev.Status() != http.StatusCreated {
		t.Errorf("Status() via Unwrap: quero 201, veio %d", ev.Status())
	}
	if !ev.Written() {
		t.Error("Written() via Unwrap deveria ser true")
	}
	if err := http.NewResponseController(cw).Flush(); err != nil {
		t.Errorf("Flush via ResponseController (SSE) falhou: %v", err)
	}
	if !rec.Flushed {
		t.Error("o Flush deveria chegar ao writer original")
	}
}

func TestDenyReason_RoundTrip(t *testing.T) {
	e, _ := newEvent(t)
	if DenyReason(e) != "" {
		t.Error("sem set, deveria ser vazio")
	}
	SetDenyReason(e, DenyNaoAprovado)
	if DenyReason(e) != DenyNaoAprovado {
		t.Error("round-trip do motivo falhou")
	}
}
