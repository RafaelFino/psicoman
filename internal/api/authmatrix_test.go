package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
)

// specOps lê o Swagger servido pelo próprio mux e devolve "METHOD path" por ator.
func specOps(t *testing.T, c *apiClient) map[string][]string {
	t.Helper()
	res := c.do(http.MethodGet, "/v1/swagger.json", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("swagger.json: %d", res.status)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Tags []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(res.body), &doc); err != nil {
		t.Fatalf("decodificando swagger: %v", err)
	}
	out := map[string][]string{}
	for path, ops := range doc.Paths {
		for method := range ops {
			var actor string
			switch {
			case strings.HasPrefix(path, "/v1/app/"):
				actor = "app"
			case strings.HasPrefix(path, "/v1/admin/"):
				actor = "admin"
			case strings.HasPrefix(path, "/v1/portal/"):
				actor = "portal"
			default:
				continue
			}
			out[actor] = append(out[actor], strings.ToUpper(method)+" "+path)
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

func splitOp(op string) (method, path string) {
	i := strings.Index(op, " ")
	return op[:i], strings.ReplaceAll(op[i+1:], "{id}", "x")
}

// TestMatrizDeAutorizacao percorre TODAS as operações documentadas e prova que,
// sem credencial, nenhuma rota protegida responde além de 401:
//   - /v1/app/* e /v1/admin/* exigem o header do Pangolin;
//   - /v1/portal/* exige sessão do paciente (única exceção: POST /register).
//
// Também prova que as credenciais não se cruzam: o header do Pangolin não abre o
// portal.
func TestMatrizDeAutorizacao(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	ops := specOps(t, c)

	if len(ops["app"]) < 45 || len(ops["portal"]) < 5 || len(ops["admin"]) < 5 {
		t.Fatalf("poucas operações lidas do swagger: app=%d portal=%d admin=%d",
			len(ops["app"]), len(ops["portal"]), len(ops["admin"]))
	}

	for _, actor := range []string{"app", "admin"} {
		for _, op := range ops[actor] {
			method, path := splitOp(op)
			if res := c.do(method, path, "{}", nil); res.status != http.StatusUnauthorized {
				t.Errorf("%s sem header do Pangolin: quero 401, veio %d (%s)", op, res.status, res.body)
			}
		}
	}

	for _, op := range ops["portal"] {
		method, path := splitOp(op)
		if method == "POST" && path == "/v1/portal/register" {
			continue // única rota aberta por desenho (auto-cadastro)
		}
		if res := c.do(method, path, "{}", nil); res.status != http.StatusUnauthorized {
			t.Errorf("%s sem sessão: quero 401, veio %d (%s)", op, res.status, res.body)
		}
		// O header do Pangolin NÃO substitui a sessão do paciente.
		if res := c.do(method, path, "{}", appHeaders()); res.status != http.StatusUnauthorized {
			t.Errorf("%s só com header do Pangolin: quero 401, veio %d", op, res.status)
		}
	}
}

// TestTokenDePacienteNaoAbreApp: a sessão de um paciente (mesmo aprovado) não dá
// acesso a /v1/app nem a /v1/admin — a identidade do terapeuta vem só do Pangolin.
func TestTokenDePacienteNaoAbreApp(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)
	pid := registerPortal(t, c, "Paciente Token", "11999990900", "token@exemplo.com")
	approvePatient(t, c, pid)
	hdr := portalAuthHeader(t, app, "token@exemplo.com")

	for _, p := range []string{"/v1/app/patients", "/v1/app/therapist", "/v1/admin/audit-log", "/v1/admin/google/status"} {
		if res := c.do(http.MethodGet, p, "", hdr); res.status != http.StatusUnauthorized {
			t.Errorf("token de paciente em %s: quero 401, veio %d", p, res.status)
		}
	}
	// Sanidade: o mesmo token abre o portal.
	if res := c.do(http.MethodGet, "/v1/portal/me", "", hdr); res.status != http.StatusOK {
		t.Errorf("token de paciente em /portal/me: quero 200, veio %d", res.status)
	}
}
