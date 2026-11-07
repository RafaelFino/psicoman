package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/integration/google"
)

// contem confere que o /metrics traz todos os trechos esperados.
func (h *metricsHarness) contem(out string, want ...string) {
	h.t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			h.t.Errorf("/metrics sem %q", w)
		}
	}
}

// --- fakes das integrações (nenhum toca a rede) ---

type okChecker struct{}

func (okChecker) HasConflict(context.Context, time.Time, time.Time) (bool, error) {
	return false, nil
}

type okCalendar struct{}

func (okCalendar) CreateEvent(context.Context, app.CalendarEvent) (*app.CalendarEventResult, error) {
	return &app.CalendarEventResult{EventID: "evt-metricas"}, nil
}

type fakeSummarizer struct{ err error }

func (f fakeSummarizer) Summarize(context.Context, app.AISummaryInput) (*app.AISummaryOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &app.AISummaryOutput{Summary: "resumo", Tags: []string{"ansiedade"}}, nil
}

func withCalendar(t *testing.T, checker app.ConflictChecker, cal app.SessionCalendar) {
	t.Helper()
	prev := calendarWirer
	calendarWirer = func(_ core.App, s *app.SessionService, _ *app.GoogleAuthService) {
		s.SetConflictChecker(checker)
		if cal != nil {
			s.SetCalendar(cal, nil)
		}
	}
	t.Cleanup(func() { calendarWirer = prev })
}

func criarPacienteESessao(t *testing.T, h *metricsHarness, email string) (pid, sid string) {
	t.Helper()
	res := h.c.do(http.MethodPost, "/v1/app/patients",
		`{"name":"Fam","phone":"11999990401","email":"`+email+`"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("criar paciente: %d (%s)", res.status, res.body)
	}
	pid = extractID(res.body)
	sres := h.c.do(http.MethodPost, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-12-21T14:00:00-03:00","ends_at":"2026-12-21T15:00:00-03:00"}`, appHeaders())
	if sres.status != http.StatusCreated {
		t.Fatalf("criar sessão: %d (%s)", sres.status, sres.body)
	}
	return pid, extractID(sres.body)
}

// TestMetricsFamilias_NegocioAuditoriaCifraFilas: o tráfego real gera os eventos
// de negócio, a trilha de auditoria, as operações de cifra e os gauges de fila
// (CountRecords real) — sem Google configurado, o agendamento conta degradado.
func TestMetricsFamilias_NegocioAuditoriaCifraFilas(t *testing.T) {
	h := withMetrics(t, nil)
	c := h.c

	// Auto-cadastro pelo portal: nasce pendente => 1 na fila.
	idP := registerPortal(t, c, "Portal", "11999990402", "portal-fam@exemplo.com")
	h.contem(h.scrape(),
		`psicoman_pacientes_eventos_total{evento="cadastrado"} 1`,
		"psicoman_fila_pacientes_pendentes 1",
	)
	approvePatient(t, c, idP)

	// Pedido de agendamento pelo portal + confirmação pelo terapeuta.
	hdr := portalAuthHeader(t, h.app, "portal-fam@exemplo.com")
	res := pedirHorario(c, hdr)
	if res.status != http.StatusCreated {
		t.Fatalf("pedido de horário: %d (%s)", res.status, res.body)
	}
	apptID := extractID(res.body)
	if r := c.do(http.MethodPost, "/v1/app/appointment-requests/"+apptID+"/confirm", "", appHeaders()); r.status != http.StatusCreated {
		t.Fatalf("confirmar pedido: %d (%s)", r.status, r.body)
	}

	// Paciente do admin + sessão + prontuário (cifra + auditoria).
	pid, sid := criarPacienteESessao(t, h, "fam-admin@exemplo.com")
	c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"texto clínico"}`, appHeaders())
	c.do(http.MethodGet, "/v1/app/patients/"+pid, "", appHeaders())
	if r := c.do(http.MethodPost, "/v1/app/sessions/"+sid+"/cancel", `{"reason":"x"}`, appHeaders()); r.status != http.StatusOK {
		t.Fatalf("cancelar: %d (%s)", r.status, r.body)
	}

	out := h.scrape()
	h.contem(out,
		`psicoman_pacientes_eventos_total{evento="cadastrado"} 2`, // portal + admin
		`psicoman_pacientes_eventos_total{evento="aprovado"} 1`,
		`psicoman_agendamentos_eventos_total{evento="criado"} 1`,
		`psicoman_agendamentos_eventos_total{evento="confirmado"} 1`,
		`psicoman_sessoes_eventos_total{evento="criada"} 2`, // do pedido confirmado + manual
		`psicoman_sessoes_eventos_total{evento="cancelada"} 1`,
		`psicoman_audit_log_total{acao="criar"}`,
		`psicoman_audit_log_total{acao="ler"}`,
		`psicoman_cifra_operacoes_total{op="seal",resultado="ok"}`,
		`psicoman_cifra_operacoes_total{op="open",resultado="ok"}`,
		// Sem Google: as sessões agendadas contam degradado (e nenhuma chamada real).
		"psicoman_calendar_degradado_total 2",
	)
	if strings.Contains(out, "psicoman_calendar_chamadas_total{") {
		t.Error("sem Google não deve haver chamadas de Calendar medidas")
	}
}

// TestMetricsFamilias_CalendarComFakes: com Calendar injetado, os decoradores
// medem freebusy e create_event (ok e conflito) e NÃO contam degradado.
func TestMetricsFamilias_CalendarComFakes(t *testing.T) {
	withCalendar(t, okChecker{}, okCalendar{})
	h := withMetrics(t, nil)
	criarPacienteESessao(t, h, "cal-ok@exemplo.com")

	out := h.scrape()
	h.contem(out,
		`psicoman_calendar_chamadas_total{op="freebusy",resultado="ok"} 1`,
		`psicoman_calendar_chamadas_total{op="create_event",resultado="ok"} 1`,
		`psicoman_calendar_duracao_segundos_count{op="freebusy"} 1`,
	)
	if strings.Contains(out, "psicoman_calendar_degradado_total 1") {
		t.Error("com Calendar injetado não deve contar degradado")
	}
}

func TestMetricsFamilias_CalendarConflito(t *testing.T) {
	withCalendar(t, conflictingChecker{}, nil)
	h := withMetrics(t, nil)

	res := h.c.do(http.MethodPost, "/v1/app/patients", `{"name":"C","phone":"11999990403","email":"conf@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)
	s := h.c.do(http.MethodPost, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-12-22T14:00:00-03:00","ends_at":"2026-12-22T15:00:00-03:00"}`, appHeaders())
	if s.status != http.StatusConflict {
		t.Fatalf("conflito: quero 409, veio %d", s.status)
	}
	h.contem(h.scrape(), `psicoman_calendar_chamadas_total{op="freebusy",resultado="conflito"} 1`)
}

// TestMetricsFamilias_Gemini: IA ligada (api_key) + chamada medida; sem a chave,
// gemini_ligada=0 e nenhuma chamada.
func TestMetricsFamilias_Gemini(t *testing.T) {
	prev := aiSummarizerBuilder
	aiSummarizerBuilder = func(core.App) app.AISummarizer { return fakeSummarizer{} }
	t.Cleanup(func() { aiSummarizerBuilder = prev })

	h := withMetrics(t, func(c *config.Config) { c.Gemini.APIKey = "SENTINELA-GEMINI-KEY" })
	_, sid := criarPacienteESessao(t, h, "ia-fam@exemplo.com")

	if r := h.c.do(http.MethodPost, "/v1/app/sessions/"+sid+"/ai-summary", `{"text":"conteudo clinico"}`, appHeaders()); r.status != http.StatusCreated && r.status != http.StatusOK {
		t.Fatalf("ai-summary: %d (%s)", r.status, r.body)
	}
	out := h.scrape()
	h.contem(out,
		"psicoman_gemini_ligada 1",
		`psicoman_gemini_chamadas_total{resultado="ok"} 1`,
		`psicoman_gemini_duracao_segundos_count{resultado="ok"} 1`,
	)
	if strings.Contains(out, "SENTINELA-GEMINI-KEY") {
		t.Error("VAZAMENTO: a api_key do Gemini apareceu em /metrics")
	}
}

func TestMetricsFamilias_GeminiDesligada(t *testing.T) {
	h := withMetrics(t, nil)
	out := h.scrape()
	if !strings.Contains(out, "psicoman_gemini_ligada 0") || strings.Contains(out, "psicoman_gemini_chamadas_total{") {
		t.Errorf("sem api_key: gemini_ligada=0 e nenhuma chamada:\n%s", out)
	}
}

// TestMetricsFamilias_BackupEDrive: backup local + espelhamento no Drive (fake)
// com retenção: contadores, tamanho, timestamp do último sucesso, drive_dr_pronto
// e snapshots mantidos; listagem com erro faz o gauge sumir.
func TestMetricsFamilias_BackupEDrive(t *testing.T) {
	fake := &google.FakeDriveClient{
		FolderID: "folder-dr", FileID: "novo",
		Files: []google.DriveFile{
			{ID: "novo", Name: "n.enc"}, {ID: "a", Name: "a.enc"},
			{ID: "b", Name: "b.enc"}, {ID: "c", Name: "c.enc"},
		},
	}
	withDriveSync(t, fake, 2)
	h := withMetrics(t, nil)

	backup(t, h.c)
	out := h.scrape()
	h.contem(out,
		`psicoman_backup_total{destino="local",resultado="ok"} 1`,
		`psicoman_backup_total{destino="drive",resultado="ok"} 1`,
		`psicoman_drive_espelhamento_total{resultado="ok"} 1`,
		"psicoman_drive_dr_pronto 1",
		"psicoman_drive_snapshots_mantidos 2",
		`psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="local"} 1.`, // epoch > 0
		`psicoman_backup_ultimo_sucesso_timestamp_segundos{destino="drive"} 1.`,
	)
	if strings.Contains(out, `psicoman_backup_tamanho_bytes{destino="drive"} 0`) {
		t.Error("o tamanho do snapshot cifrado deveria ser > 0")
	}

	// Listagem falha: gauge ausente + falha contada, espelhamento segue ok.
	fake.ListErr = errors.New("403")
	backup(t, h.c)
	out = h.scrape()
	if strings.Contains(out, "psicoman_drive_snapshots_mantidos") {
		t.Error("listagem com erro deve tornar drive_snapshots_mantidos ausente")
	}
	h.contem(out, `psicoman_drive_retencao_falhas_total{etapa="listar"} 1`)

	// Upload falha: DR deixa de estar pronto.
	fake.UploadErr = errors.New("rede")
	backup(t, h.c)
	h.contem(h.scrape(), "psicoman_drive_dr_pronto 0", `psicoman_drive_espelhamento_total{resultado="erro"} 1`)
}

// TestMetricsFamilias_RateLimit: o 429 do cadastro é contado por escopo.
func TestMetricsFamilias_RateLimit(t *testing.T) {
	h := withMetrics(t, nil)
	corpo := `{"name":"RL","phone":"11999990404","email":"rl@exemplo.com","password":"senha-forte-123"}`
	for i := 0; i < 5; i++ {
		if r := h.c.do(http.MethodPost, "/v1/portal/register", corpo, nil); r.status == http.StatusTooManyRequests {
			t.Fatalf("tentativa %d não pode ser 429", i+1)
		}
	}
	if r := h.c.do(http.MethodPost, "/v1/portal/register", corpo, nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("6ª tentativa: quero 429, veio %d", r.status)
	}
	h.contem(h.scrape(), `psicoman_rate_limit_rejeicoes_total{escopo="cadastro"} 1`)
}

// TestMetricsFamilias_GoogleOAuthCoerenteComReadyz: google_oauth_ok acompanha o
// MESMO critério do /readyz (0 degradado -> 1 após autorizar) e nenhum segredo
// do OAuth (client_secret, refresh token) vaza em /metrics.
func TestMetricsFamilias_GoogleOAuthCoerenteComReadyz(t *testing.T) {
	srv := fakeOAuthServer(t, "1//refresh-token-sentinela-metricas")
	t.Cleanup(google.SetAuthEndpointForTest(srv.URL+"/authorize", srv.URL+"/token"))
	h := withMetrics(t, func(cfg *config.Config) {
		cfg.Google = config.GoogleConfig{ClientID: "test-client-id", ClientSecret: "client-secret-sentinela"}
	})

	h.contem(h.scrape(), "psicoman_google_oauth_ok 0")
	if r := h.c.do(http.MethodGet, "/v1/readyz", "", nil); !strings.Contains(r.body, `"google":"degraded"`) {
		t.Fatalf("readyz deveria estar degradado: %s", r.body)
	}

	if cb := autorizarGoogle(t, h.c); cb.status != http.StatusOK {
		t.Fatalf("autorizar: %d (%s)", cb.status, cb.body)
	}
	out := h.scrape()
	h.contem(out, "psicoman_google_oauth_ok 1")
	if r := h.c.do(http.MethodGet, "/v1/readyz", "", nil); !strings.Contains(r.body, `"google":"ok"`) {
		t.Errorf("readyz deveria estar ok após autorizar: %s", r.body)
	}
	for _, segredo := range []string{"1//refresh-token-sentinela-metricas", "client-secret-sentinela", "test-client-id"} {
		if strings.Contains(out, segredo) {
			t.Errorf("VAZAMENTO em /metrics: %q", segredo)
		}
	}
}
