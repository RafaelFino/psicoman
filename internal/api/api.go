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
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"

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

		// Hook de reforço de aprovação (defesa em profundidade, Fase 5): barra
		// escrita de appointment_request de paciente não-aprovado por qualquer
		// caminho (inclusive o adapter), complementando o gate do portal.
		registerApprovalHooks(e.App)

		// Nome do header de identidade do Pangolin, resolvido uma vez no boot.
		pangolinHdr := httpx.PangolinHeaderName()

		// Bypass de identidade de DEV (pangolin.dev_identity): relaxa o gate
		// quando setado. Resolvido via configLoader (mesma fonte do resto do
		// wiring). Erro ao ler config → devIdentity "" (fail-closed: não relaxa).
		var devIdentity string
		if c, _, cerr := configLoader(); cerr == nil {
			devIdentity = strings.TrimSpace(c.Pangolin.DevIdentity)
		}
		if devIdentity != "" {
			// WARN visível no boot; loga só o nome da chave, NUNCA o valor.
			e.App.Logger().Warn(
				"ATENÇÃO: modo dev de identidade ATIVO — gate do Pangolin relaxado, NÃO usar em produção",
				"campo", "pangolin.dev_identity")
		}

		// Métricas HTTP (R8): middleware no router RAIZ, para medir também o 404
		// de rota inexistente (catch-all) e as rotas /api/* do PocketBase.
		// Priority -5000 < -1040 (activity logger do PB): observa inclusive as
		// respostas curtas dos middlewares padrão. Falha de métrica nunca vira 500.
		e.Router.Bind(&hook.Handler[*core.RequestEvent]{
			Id:       "psicoman_http_metrics",
			Priority: -5000,
			Func:     httpx.Instrument(svc.Obs.HTTP()),
		})

		v1 := e.Router.Group("/v1")
		v1.BindFunc(httpx.Recover, httpx.RequestID, httpx.Timing, httpx.Logging)

		// Health/observabilidade no envelope, fora da auth (probes de operação).
		v1.GET("/health", apiapp.Health)
		v1.GET("/livez", livez)
		v1.GET("/readyz", readyz(svc))

		// Swagger (um documento /v1 com os três atores por tags).
		swagger.Mount(v1)

		appGrp := v1.Group("/app")
		appGrp.BindFunc(httpx.RequirePangolinIdentity(pangolinHdr, devIdentity), httpx.PropagateActor)
		apiapp.Mount(appGrp, svc)

		portalGrp := v1.Group("/portal")
		portalGrp.BindFunc(portal.RequireApproved(svc))
		portal.Mount(portalGrp, svc)

		adminGrp := v1.Group("/admin")
		adminGrp.BindFunc(httpx.RequirePangolinIdentity(pangolinHdr, devIdentity), httpx.PropagateActor)
		admin.Mount(adminGrp, svc)

		// Listener dedicado de métricas (R8.1): só sobe com metrics.addr. Falha de
		// bind aborta o boot (config deliberada). A API NÃO serve /metrics.
		var metricsAddr string
		if c, _, cerr := configLoader(); cerr == nil {
			metricsAddr = c.Metrics.Addr
		}
		if err := StartMetricsServer(e.App, metricsAddr, svc.Obs.Handler(), e.App.Logger()); err != nil {
			return err
		}

		return e.Next()
	})
}
