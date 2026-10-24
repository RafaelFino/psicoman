// Package swagger serve a documentação OpenAPI/Swagger do contrato /v1 (um único
// documento com os três atores por tags: app/portal/admin). A spec é embutida
// via //go:embed e servida em GET /v1/swagger.json; a UI (Swagger UI via CDN)
// em GET /v1/swagger.
package swagger

import (
	_ "embed"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

//go:embed openapi.base.json
var baseSpec []byte

// Mount registra /v1/swagger (UI) e /v1/swagger.json (spec) no grupo /v1.
func Mount(g *router.RouterGroup[*core.RequestEvent]) {
	g.GET("/swagger.json", func(e *core.RequestEvent) error {
		return e.Blob(http.StatusOK, "application/json; charset=utf-8", baseSpec)
	})
	g.GET("/swagger", func(e *core.RequestEvent) error {
		return e.HTML(http.StatusOK, swaggerHTML)
	})
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
    window.ui = SwaggerUIBundle({ url: "/v1/swagger.json", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`
