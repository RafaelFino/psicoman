package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/app"

	// Blank import registra as migrations do sistema (PocketBase), necessárias
	// para o Bootstrap criar as tabelas internas (_collections, etc.).
	_ "github.com/pocketbase/pocketbase/migrations"
)

// newEvent monta um *core.RequestEvent mínimo sobre um BaseApp em processo +
// httptest.ResponseRecorder, para exercitar os helpers de envelope/erro sem
// subir o app inteiro (padrão de pb_test.go).
func newEvent(t *testing.T) (*core.RequestEvent, *httptest.ResponseRecorder) {
	t.Helper()
	a := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := a.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if err := a.RunAllMigrations(); err != nil {
		t.Fatalf("RunAllMigrations: %v", err)
	}
	t.Cleanup(func() { _ = a.ResetBootstrapState() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	e := &core.RequestEvent{App: a}
	e.Event = router.Event{Response: rec, Request: req}
	return e, rec
}

// TestEnvelopeOrdemSucesso: RespondOK → error null, elapsed_ms presente, e as 4
// chaves na ordem contratual {message, elapsed_ms, data, error} (R7.2). A ordem
// é afirmada por índice de substring no corpo bruto — detecta regressão caso
// alguém troque a struct Envelope por map[string]any (correção finding-3).
func TestEnvelopeOrdemSucesso(t *testing.T) {
	e, rec := newEvent(t)
	if err := RespondOK(e, "ok", map[string]any{"status": "ok"}); err != nil {
		t.Fatalf("RespondOK: %v", err)
	}
	body := rec.Body.String()

	iMsg := strings.Index(body, `"message"`)
	iElapsed := strings.Index(body, `"elapsed_ms"`)
	iData := strings.Index(body, `"data"`)
	iErr := strings.Index(body, `"error"`)
	if iMsg < 0 || iElapsed < 0 || iData < 0 || iErr < 0 {
		t.Fatalf("chaves ausentes no corpo: %s", body)
	}
	if !(iMsg < iElapsed && iElapsed < iData && iData < iErr) {
		t.Errorf("ordem das chaves do envelope incorreta: %s", body)
	}
	if !strings.Contains(body, `"error":null`) {
		t.Errorf("esperava error:null em sucesso: %s", body)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status: quero 200, veio %d", rec.Code)
	}
}

func TestFailMapeamentoNotFound(t *testing.T) {
	e, rec := newEvent(t)
	if err := Fail(e, app.ErrNotFound); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: quero 404, veio %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"nao_encontrado"`) {
		t.Errorf("code esperado nao_encontrado: %s", body)
	}
	if !strings.Contains(body, `"data":null`) {
		t.Errorf("esperava data:null em erro: %s", body)
	}
}

func TestFailMapeamentoValidacao(t *testing.T) {
	e, rec := newEvent(t)
	if err := Fail(e, app.NewValidation("O conteúdo da nota é obrigatório.")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: quero 422, veio %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"validacao"`) {
		t.Errorf("code esperado validacao: %s", body)
	}
	if !strings.Contains(body, "O conteúdo da nota é obrigatório.") {
		t.Errorf("message PT-BR do serviço esperada: %s", body)
	}
}

func TestFailMapeamentoConflito(t *testing.T) {
	e, rec := newEvent(t)
	if err := Fail(e, app.NewConflict("Já existe um paciente com este email.")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("status: quero 409, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"conflito"`) {
		t.Errorf("code esperado conflito: %s", rec.Body.String())
	}
}

// TestFailErroInternoNaoVazaDetalhe: erro não mapeado → 500 erro_interno, SEM o
// detalhe técnico no corpo (só no log).
func TestFailErroInternoNaoVazaDetalhe(t *testing.T) {
	e, rec := newEvent(t)
	detalhe := "pq: connection refused segredo_interno"
	if err := Fail(e, errors.New(detalhe)); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: quero 500, veio %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "segredo_interno") {
		t.Errorf("detalhe interno vazou no corpo: %s", body)
	}
	if !strings.Contains(body, `"code":"erro_interno"`) {
		t.Errorf("code esperado erro_interno: %s", body)
	}
}

// TestFailBadRequest: corpo malformado → 400 requisicao_invalida (não 500).
func TestFailBadRequest(t *testing.T) {
	e, rec := newEvent(t)
	if err := FailBadRequest(e, errors.New("unexpected EOF")); err != nil {
		t.Fatalf("FailBadRequest: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: quero 400, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"requisicao_invalida"`) {
		t.Errorf("code esperado requisicao_invalida: %s", rec.Body.String())
	}
}

func TestFmtTimeZero(t *testing.T) {
	if got := FmtTime(time.Time{}); got != "" {
		t.Errorf("instante zero deve virar \"\", veio %q", got)
	}
}
