// Package swagger serve a documentação OpenAPI/Swagger dos dois binários,
// com escopo por namespace (docs/architecture.md §5).
//
// A spec base é embutida e servida em /v1/swagger.json; uma página HTML mínima
// (Swagger UI via CDN) é servida em /v1/swagger.
package swagger

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed openapi.base.json
var baseSpec []byte

// Surface identifica qual conjunto de rotas documentar.
type Surface string

const (
	// Admin documenta o namespace /v1/admin.
	Admin Surface = "admin"
	// Portal documenta o namespace /v1/portal.
	Portal Surface = "portal"
)

// Register instala /v1/swagger (UI) e /v1/swagger.json (spec) no mux.
func Register(mux *http.ServeMux, surface Surface) {
	mux.HandleFunc("GET /v1/swagger.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(specFor(surface))
	})
	mux.HandleFunc("GET /v1/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerHTML))
	})
}

// specFor devolve a spec ajustada para a superfície (título/descrição).
func specFor(surface Surface) []byte {
	title := "Psicoman — API Admin"
	if surface == Portal {
		title = "Psicoman — API Portal"
	}
	s := strings.ReplaceAll(string(baseSpec), "__TITLE__", title)
	return []byte(s)
}

const swaggerHTML = `<!doctype html>
<html lang="pt-br">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Psicoman — Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/v1/swagger.json',
        dom_id: '#swagger-ui',
      });
    };
  </script>
</body>
</html>`
