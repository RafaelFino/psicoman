package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/RafaelFino/psicoman/internal/platform/crypto"

	// Blank import registra as migrations do schema (coleções do MER).
	_ "github.com/RafaelFino/psicoman/internal/platform/pb/migrations"
)

// testKeyB64 é a mesma chave AES-256 fixa de pb_test.go (determinística).
const testKeyB64 = "wPb6Yex4abAI5VpFY6SIonnjtPOmtuGP2MJVnezqUKY="

// appHeaders devolve o header de identidade do Pangolin exigido por /v1/app/* e
// /v1/admin/* a partir da Fase 5 (default X-Pangolin-User).
func appHeaders() map[string]string {
	return map[string]string{"X-Pangolin-User": "terapeuta@exemplo.com"}
}

// withTestCipher troca o loader de config de cifra por um determinístico (sem
// depender de env vars globais) e devolve uma função de restauração.
func withTestCipher(t *testing.T) {
	t.Helper()
	prev := cipherConfigLoader
	cipherConfigLoader = func() (cipherConfig, error) {
		return cipherConfig{
			CurrentKeyID: "k1",
			Keys:         []crypto.KeySpec{{ID: "k1", B64Key: testKeyB64}},
		}, nil
	}
	t.Cleanup(func() { cipherConfigLoader = prev })
}

// newTestApp cria um TestApp com a cifra de teste injetada e api.Register
// aplicado (padrão da factory do design, §10/N1).
func newTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	withTestCipher(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	Register(app)
	return app
}

// apiClient é um cliente em processo sobre o mux do OnServe, permitindo fluxos
// multi-passo (seed via API + leitura) sem subir socket.
type apiClient struct {
	t   *testing.T
	mux http.Handler
}

// newClient monta o mux do OnServe uma vez e devolve um cliente reutilizável.
func newClient(t *testing.T, app *tests.TestApp) *apiClient {
	t.Helper()
	baseRouter, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	serveEvent := new(core.ServeEvent)
	serveEvent.App = app
	serveEvent.Router = baseRouter
	var mux http.Handler
	err = app.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
		m, berr := e.Router.BuildMux()
		if berr != nil {
			return berr
		}
		mux = m
		return nil
	})
	if err != nil {
		t.Fatalf("Trigger OnServe: %v", err)
	}
	return &apiClient{t: t, mux: mux}
}

type apiResp struct {
	status int
	body   string
}

func (c *apiClient) do(method, url, body string, headers map[string]string) apiResp {
	c.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, url, r)
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	c.mux.ServeHTTP(rec, req)
	return apiResp{status: rec.Code, body: rec.Body.String()}
}

// extractID lê um id "01..." do corpo (data.id ou patient.id). Simplista: busca
// o primeiro "id":"...". Suficiente para os testes de contrato.
func extractID(body string) string {
	const key = `"id":"`
	i := strings.Index(body, key)
	if i < 0 {
		return ""
	}
	rest := body[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestHealthEnvelope: GET /v1/health → 200 no envelope.
func TestHealthEnvelope(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/health", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("status: quero 200, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"elapsed_ms"`) || !strings.Contains(res.body, `"error":null`) {
		t.Errorf("health não está no envelope: %s", res.body)
	}
}

// TestProntuarioLegivelNoApp (4.1b): POST paciente + POST nota "X" → GET
// paciente devolve a nota em claro (via Reveal), provando o caminho terapeuta.
func TestProntuarioLegivelNoApp(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)

	const claro = "paciente relatou melhora do sono SEGREDO_CLINICO"

	res := c.do(http.MethodPost, "/v1/app/patients", `{"name":"Maria","phone":"11999990001","email":"maria@exemplo.com"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", res.status, res.body)
	}
	pid := extractID(res.body)
	if pid == "" {
		t.Fatalf("id do paciente não extraído: %s", res.body)
	}

	res = c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"`+claro+`"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("criar nota: status %d (%s)", res.status, res.body)
	}

	res = c.do(http.MethodGet, "/v1/app/patients/"+pid, "", appHeaders())
	if res.status != http.StatusOK {
		t.Fatalf("get chart: status %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, claro) {
		t.Errorf("o terapeuta deveria ler a nota em claro: %s", res.body)
	}
}

// TestNoteContentNaoVazaCifradoNoBanco: a coluna crua da nota guarda o envelope
// cifrado, nunca o claro (D6.3).
func TestNoteContentNaoVazaCifradoNoBanco(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const claro = "conteudo clinico NUNCA_EM_CLARO_NO_BANCO"

	res := c.do(http.MethodPost, "/v1/app/patients", `{"name":"Ana","phone":"11999990002","email":"ana@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)
	c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"`+claro+`"}`, appHeaders())

	rec, err := app.FindFirstRecordByFilter("note", "patient_id = {:p}", map[string]any{"p": pid})
	if err != nil {
		t.Fatalf("buscar nota crua: %v", err)
	}
	raw := rec.GetString("conteudo")
	if raw == "" {
		t.Fatalf("coluna conteudo vazia")
	}
	if strings.Contains(raw, claro) {
		t.Errorf("o claro vazou na coluna crua: %q", raw)
	}
	if !strings.HasPrefix(raw, "v1.aes-256-gcm.") {
		t.Errorf("coluna crua não está no envelope de cifra: %q", raw)
	}
}

// TestPortalSemClinico (4.1b): GET /v1/portal/me e /sessions NÃO contêm o claro
// clínico inserido no prontuário.
func TestPortalSemClinico(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const claro = "anotacao clinica SEGREDO_PORTAL_NAO_VE"

	// Cadastra pelo portal (cria a credencial patient_user) e aprova, para ter
	// uma sessão REAL (Fase 9) ao ler /me e /sessions.
	const email = "joao@exemplo.com"
	pid := registerPortal(t, c, "Joao", "11999990003", email)
	approvePatient(t, c, pid)
	c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"`+claro+`"}`, appHeaders())

	hdr := portalAuthHeader(t, app, email)
	me := c.do(http.MethodGet, "/v1/portal/me", "", hdr)
	if me.status != http.StatusOK {
		t.Fatalf("portal/me: status %d (%s)", me.status, me.body)
	}
	if strings.Contains(me.body, claro) || strings.Contains(me.body, "content") {
		t.Errorf("portal/me não pode conter clínico: %s", me.body)
	}
	ss := c.do(http.MethodGet, "/v1/portal/sessions", "", hdr)
	if ss.status != http.StatusOK {
		t.Fatalf("portal/sessions: status %d (%s)", ss.status, ss.body)
	}
	if strings.Contains(ss.body, claro) {
		t.Errorf("portal/sessions não pode conter clínico: %s", ss.body)
	}
}

// TestRotasPorAtorNoEnvelope: rotas principais respondem no envelope com o
// status esperado.
func TestRotasPorAtorNoEnvelope(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)

	// cria paciente para ter um id de sessão/debts.
	res := c.do(http.MethodPost, "/v1/app/patients", `{"name":"Bia","phone":"11999990004","email":"bia@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)

	// POST sessão → 201.
	sres := c.do(http.MethodPost, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-10-21T14:00:00-03:00","ends_at":"2026-10-21T15:00:00-03:00"}`, appHeaders())
	if sres.status != http.StatusCreated {
		t.Errorf("POST sessions: quero 201, veio %d (%s)", sres.status, sres.body)
	}

	// GET débitos do paciente → 200 (lista vazia no envelope).
	dres := c.do(http.MethodGet, "/v1/app/patients/"+pid+"/debts", "", appHeaders())
	if dres.status != http.StatusOK || !strings.Contains(dres.body, `"elapsed_ms"`) {
		t.Errorf("GET debts: quero 200 no envelope, veio %d (%s)", dres.status, dres.body)
	}

	// POST backup (admin) → 202 (backup real; hook de DR no-op sem Google).
	bres := c.do(http.MethodPost, "/v1/admin/backup", "", appHeaders())
	if bres.status != http.StatusAccepted || !strings.Contains(bres.body, `"status":"ok"`) {
		t.Errorf("POST backup: quero 202 ok, veio %d (%s)", bres.status, bres.body)
	}

	// GET audit-log (admin) → 200 no envelope.
	ares := c.do(http.MethodGet, "/v1/admin/audit-log", "", appHeaders())
	if ares.status != http.StatusOK || !strings.Contains(ares.body, `"elapsed_ms"`) {
		t.Errorf("GET audit-log: quero 200 no envelope, veio %d (%s)", ares.status, ares.body)
	}
}

// TestBindBodyInvalidoVira400: corpo malformado → 400 requisicao_invalida.
func TestBindBodyInvalidoVira400(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodPost, "/v1/app/patients", `{isso nao e json`, appHeaders())
	if res.status != http.StatusBadRequest {
		t.Fatalf("quero 400, veio %d (%s)", res.status, res.body)
	}
	if !strings.Contains(res.body, `"code":"requisicao_invalida"`) {
		t.Errorf("code esperado requisicao_invalida: %s", res.body)
	}
}

// TestNotaContentObrigatorioVira422: content vazio → 422 validacao.
func TestNotaContentObrigatorioVira422(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodPost, "/v1/app/patients", `{"name":"Cris","phone":"11999990005","email":"cris@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)
	nres := c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":""}`, appHeaders())
	if nres.status != http.StatusUnprocessableEntity {
		t.Fatalf("quero 422, veio %d (%s)", nres.status, nres.body)
	}
	if !strings.Contains(nres.body, `"code":"validacao"`) {
		t.Errorf("code esperado validacao: %s", nres.body)
	}
}

// TestPendingNaoEhTratadoComoID: GET /v1/app/patients/pending cai no handler de
// fila (não é tratado como {id}="pending" → 404). Prova a precedência por
// especificidade do http.ServeMux.
func TestPendingNaoEhTratadoComoID(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/v1/app/patients/pending", "", appHeaders())
	if res.status != http.StatusOK {
		t.Fatalf("pending: quero 200 (fila), veio %d (%s)", res.status, res.body)
	}
}

// TestRESTAutomaticaTrancada (D13): GET /api/collections/note/records sem
// superuser → 403 (a REST não é contrato público; regra nil).
func TestRESTAutomaticaTrancada(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	res := c.do(http.MethodGet, "/api/collections/note/records", "", nil)
	if res.status != http.StatusForbidden {
		t.Errorf("REST sem superuser: quero 403, veio %d (%s)", res.status, res.body)
	}
}

// newSuperuserToken cria um superuser determinístico e devolve um token de auth
// válido para a REST automática do PocketBase (usado no caso D13 de superuser).
func newSuperuserToken(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("coleção _superusers: %v", err)
	}
	su := core.NewRecord(col)
	su.SetEmail("root@exemplo.com")
	su.SetPassword("1234567890")
	if err := app.Save(su); err != nil {
		t.Fatalf("salvar superuser: %v", err)
	}
	tok, err := su.NewAuthToken()
	if err != nil {
		t.Fatalf("token de superuser: %v", err)
	}
	return tok
}

// TestRESTSuperuserDevolveCifrado (D13, §6/§10 caso 6): mesmo autenticado como
// superuser, a REST automática devolve o campo clínico como ENVELOPE CIFRADO —
// nunca o claro. Prova que a REST não é via de vazamento mesmo com credencial
// máxima.
func TestRESTSuperuserDevolveCifrado(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	const claro = "conteudo clinico SUPERUSER_NAO_VE_CLARO_NA_REST"

	res := c.do(http.MethodPost, "/v1/app/patients", `{"name":"Lia","phone":"11999990006","email":"lia@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)
	if pid == "" {
		t.Fatalf("id do paciente não extraído: %s", res.body)
	}
	nres := c.do(http.MethodPost, "/v1/app/patients/"+pid+"/notes", `{"content":"`+claro+`"}`, appHeaders())
	if nres.status != http.StatusCreated {
		t.Fatalf("criar nota: status %d (%s)", nres.status, nres.body)
	}

	tok := newSuperuserToken(t, app)
	hdr := map[string]string{"Authorization": tok}
	rest := c.do(http.MethodGet, "/api/collections/note/records", "", hdr)
	if rest.status != http.StatusOK {
		t.Fatalf("REST com superuser: quero 200, veio %d (%s)", rest.status, rest.body)
	}
	// O corpo deve conter a nota (há 1 registro) mas com o campo clínico cifrado.
	if strings.Contains(rest.body, claro) {
		t.Errorf("o claro vazou na REST autenticada como superuser: %s", rest.body)
	}
	if !strings.Contains(rest.body, "v1.aes-256-gcm.") {
		t.Errorf("REST não devolveu o campo clínico no envelope de cifra: %s", rest.body)
	}
}

// TestSwaggerServido: /v1/swagger.json e /v1/swagger respondem 200.
func TestSwaggerServido(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	j := c.do(http.MethodGet, "/v1/swagger.json", "", nil)
	if j.status != http.StatusOK || !strings.Contains(j.body, `"openapi"`) {
		t.Errorf("swagger.json: quero 200 JSON, veio %d", j.status)
	}
	u := c.do(http.MethodGet, "/v1/swagger", "", nil)
	if u.status != http.StatusOK || !strings.Contains(u.body, "swagger-ui") {
		t.Errorf("swagger UI: quero 200 HTML, veio %d", u.status)
	}
}
