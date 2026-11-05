package swagger

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// routeRe captura registros do tipo g.GET("/x", ...) e v1.GET("/x", ...).
var routeRe = regexp.MustCompile(`\b(?:g|v1)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]+)"`)

// registered lê um arquivo de montagem e devolve "METHOD /prefixo/path".
func registered(t *testing.T, file, prefix string) []string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("lendo %s: %v", file, err)
	}
	var out []string
	for _, m := range routeRe.FindAllStringSubmatch(string(src), -1) {
		out = append(out, m[1]+" "+prefix+m[2])
	}
	return out
}

type operation struct {
	Tags      []string         `json:"tags"`
	Summary   string           `json:"summary"`
	Security  []map[string]any `json:"security"`
	Responses map[string]any   `json:"responses"`
}

func loadSpec(t *testing.T) map[string]map[string]operation {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]operation `json:"paths"`
	}
	if err := json.Unmarshal(baseSpec, &doc); err != nil {
		t.Fatalf("openapi.base.json inválido: %v", err)
	}
	return doc.Paths
}

// TestSwaggerBateComAsRotasRegistradas é o contrato de documentação: toda rota
// montada precisa estar no Swagger e toda operação do Swagger precisa existir.
func TestSwaggerBateComAsRotasRegistradas(t *testing.T) {
	var routes []string
	routes = append(routes, registered(t, "../app/mount.go", "/v1/app")...)
	routes = append(routes, registered(t, "../portal/me.go", "/v1/portal")...)
	routes = append(routes, registered(t, "../admin/ops.go", "/v1/admin")...)
	routes = append(routes, registered(t, "../api.go", "/v1")...)
	if len(routes) < 60 {
		t.Fatalf("extração de rotas suspeita: só %d encontradas", len(routes))
	}

	spec := loadSpec(t)
	documented := map[string]bool{}
	for path, ops := range spec {
		for method := range ops {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	registeredSet := map[string]bool{}
	for _, r := range routes {
		registeredSet[r] = true
	}

	// Rotas servidas fora de mount (swagger.Mount) e que o spec não precisa listar.
	ignoreUndocumented := map[string]bool{"GET /v1/swagger": true, "GET /v1/swagger.json": true}

	var missing, orphan []string
	for r := range registeredSet {
		if !documented[r] && !ignoreUndocumented[r] {
			missing = append(missing, r)
		}
	}
	for d := range documented {
		if !registeredSet[d] {
			orphan = append(orphan, d)
		}
	}
	sort.Strings(missing)
	sort.Strings(orphan)
	if len(missing) > 0 {
		t.Errorf("rotas registradas SEM documentação no Swagger:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(orphan) > 0 {
		t.Errorf("operações no Swagger SEM rota registrada:\n  %s", strings.Join(orphan, "\n  "))
	}
}

// TestSwaggerOperacoesBemFormadas: toda operação tem tag de ator coerente com o
// prefixo, resumo, respostas e (exceto rotas abertas) declaração de segurança.
func TestSwaggerOperacoesBemFormadas(t *testing.T) {
	open := map[string]bool{
		"/v1/health": true, "/v1/livez": true, "/v1/readyz": true,
		"/v1/portal/register": true,
	}
	for path, ops := range loadSpec(t) {
		for method, op := range ops {
			id := strings.ToUpper(method) + " " + path
			if op.Summary == "" {
				t.Errorf("%s: sem summary", id)
			}
			if len(op.Responses) == 0 {
				t.Errorf("%s: sem responses", id)
			}
			if len(op.Tags) != 1 {
				t.Errorf("%s: deveria ter exatamente 1 tag, tem %v", id, op.Tags)
				continue
			}
			switch {
			case strings.HasPrefix(path, "/v1/app/") && op.Tags[0] != "app":
				t.Errorf("%s: tag %q incoerente com o prefixo", id, op.Tags[0])
			case strings.HasPrefix(path, "/v1/portal/") && op.Tags[0] != "portal":
				t.Errorf("%s: tag %q incoerente com o prefixo", id, op.Tags[0])
			case strings.HasPrefix(path, "/v1/admin/") && op.Tags[0] != "admin":
				t.Errorf("%s: tag %q incoerente com o prefixo", id, op.Tags[0])
			}
			if !open[path] && len(op.Security) == 0 {
				t.Errorf("%s: rota protegida sem 'security' no Swagger", id)
			}
		}
	}
}
