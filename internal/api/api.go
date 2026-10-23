// Package api registra as rotas próprias /v1/* do Psicoman sobre o router do
// PocketBase (via app.OnServe()).
//
// Segregação lógica por ator, em paths distintos, desenhada para o Pangolin
// aplicar controle de acesso por prefixo (D2):
//
//   - /v1/app/*    terapeuta (também a operadora/"admin" no single-tenant)
//   - /v1/portal/* paciente (aberto: cadastro/login + recursos mínimos)
//   - /v1/admin/*  operação técnica da instância
//
// A REST automática do PocketBase NÃO é o contrato público: o contrato são as
// rotas /v1/* próprias com o envelope padrão { message, elapsed_ms, data, error }
// (ver Fase 4 / D7). As coleções do MER têm regras nulas (somente superuser) e o
// conteúdo clínico fica cifrado em repouso (D6/D13).
//
// Esta é a camada mais externa: depende de internal/app e internal/platform;
// nunca o contrário.
package api

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/admin"
	apiapp "github.com/RafaelFino/psicoman/internal/api/app"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/portal"
	"github.com/RafaelFino/psicoman/internal/api/swagger"
)

// Register pendura as rotas próprias /v1/* no hook OnServe do PocketBase.
//
// Monta os serviços (repos + cipher + serviços + hooks de sessão) e registra os
// grupos por ator com a middleware chain comum. Falha de montagem (ex.: config
// de cifra ausente) aborta o boot — não subir o servidor sem cifra.
func Register(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		svc, err := buildServices(e.App)
		if err != nil {
			return err
		}

		v1 := e.Router.Group("/v1")
		v1.BindFunc(httpx.Recover, httpx.RequestID, httpx.Timing, httpx.Logging)

		// Health no envelope (mantém 200 para o smoke de boot).
		v1.GET("/health", apiapp.Health)

		// Swagger (um documento /v1 com os três atores por tags).
		swagger.Mount(v1)

		appGrp := v1.Group("/app")
		appGrp.BindFunc(httpx.AuthAppStub)
		apiapp.Mount(appGrp, svc)

		portalGrp := v1.Group("/portal")
		portalGrp.BindFunc(httpx.AuthPortalStub)
		portal.Mount(portalGrp, svc)

		adminGrp := v1.Group("/admin")
		adminGrp.BindFunc(httpx.AuthAdminStub)
		admin.Mount(adminGrp, svc)

		return e.Next()
	})
}
