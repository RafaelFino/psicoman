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
// (ver Fase 4 / D7).
//
// Esta é a camada mais externa: depende de internal/app; nunca o contrário.
package api

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

// Register pendura as rotas próprias /v1/* no hook OnServe do PocketBase.
//
// Na Fase 0 registra apenas o health check /v1/health (para o smoke de boot);
// os grupos por ator e as rotas de negócio chegam nas fases seguintes.
func Register(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		v1 := e.Router.Group("/v1")

		v1.GET("/health", func(re *core.RequestEvent) error {
			return re.JSON(http.StatusOK, map[string]any{
				"message":    "Psicoman no ar.",
				"elapsed_ms": 0,
				"data":       map[string]any{"status": "ok"},
				"error":      nil,
			})
		})

		return e.Next()
	})
}
