package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
)

// scrape devolve o texto de exposição do Handler.
func scrape(t *testing.T, o *Obs) string {
	t.Helper()
	rec := httptest.NewRecorder()
	o.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape: quero 200, veio %d", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	return string(b)
}

func newObs(dev bool) *Obs {
	return New(Config{Build: BuildInfo{Version: "v-teste", GoVersion: "go1.27"}, DevIdentity: dev})
}

func TestObs_ExposicaoBase(t *testing.T) {
	out := scrape(t, newObs(false))
	for _, want := range []string{
		"go_goroutines",
		"process_open_fds",
		`psicoman_build_info{go="go1.27",versao="v-teste"} 1`,
		"psicoman_uptime_segundos",
		"psicoman_identidade_dev_ativa 0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposição sem %q", want)
		}
	}
}

func TestObs_IdentidadeDevAtiva(t *testing.T) {
	if out := scrape(t, newObs(true)); !strings.Contains(out, "psicoman_identidade_dev_ativa 1") {
		t.Error("identidade_dev_ativa deveria ser 1 com dev_identity")
	}
}

func TestObs_RegistryProprioNaoPolui(t *testing.T) {
	// Dois Obs no mesmo processo não colidem (registry próprio, não o default).
	a, b := newObs(false), newObs(false)
	a.HTTP().RateLimited("cadastro")
	if strings.Contains(scrape(t, b), "rate_limit_rejeicoes_total{") {
		t.Error("o registry do segundo Obs não deveria ver a série do primeiro")
	}
}

func TestHTTPMetrics_Familias(t *testing.T) {
	o := newObs(false)
	h := o.HTTP()
	h.InFlight("app", +1)
	h.Request("app", "/v1/app/patients/{id}", "GET", 404, 120*time.Millisecond, 512)
	h.Request("portal", "nao_encontrada", "GET", 404, time.Millisecond, -1) // tamanho desconhecido
	h.Denied("app", 401, "sem_identidade")
	h.RateLimited("cadastro")

	out := scrape(t, o)
	for _, want := range []string{
		`psicoman_http_requisicoes_total{ator="app",metodo="GET",rota="/v1/app/patients/{id}",status="404"} 1`,
		`psicoman_http_duracao_segundos_count{ator="app",metodo="GET",rota="/v1/app/patients/{id}"} 1`,
		`psicoman_http_em_andamento{ator="app"} 1`,
		`psicoman_http_resposta_bytes_count{ator="app",rota="/v1/app/patients/{id}"} 1`,
		`psicoman_seguranca_negados_total{ator="app",motivo="sem_identidade",status="401"} 1`,
		`psicoman_rate_limit_rejeicoes_total{escopo="cadastro"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposição sem %q", want)
		}
	}
	// Tamanho desconhecido (-1) não gera amostra de tamanho.
	if strings.Contains(out, `psicoman_http_resposta_bytes_count{ator="portal"`) {
		t.Error("bytes < 0 não deveria observar o histograma de tamanho")
	}
	h.InFlight("app", -1)
	if !strings.Contains(scrape(t, o), `psicoman_http_em_andamento{ator="app"} 0`) {
		t.Error("em_andamento deveria voltar a 0")
	}
}

func TestObs_App_EhNopNaFase7(t *testing.T) {
	// A porta de negócio existe e é segura de chamar (famílias na Fase 8).
	m := newObs(false).App()
	m.Patient(app.PatientRegistered)
	m.CipherOp(app.CipherSeal, app.ResultOK)
}

func TestDriveMetrics_NaoPanica(t *testing.T) {
	d := newObs(false).Drive()
	d.Backup("local", true, time.Second, 10)
	d.Mirror("ok")
	d.Retention(3)
	d.RetentionFailure("listar")
}
