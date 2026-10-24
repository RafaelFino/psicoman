package httpx

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// Ordem da chain (R7): recover → request-id → timing → logging → auth → handler.
// recover/request-id/timing/logging ficam no grupo /v1 (valem para todos); auth
// fica no subgrupo de cada ator (política por ator, D2). A auth real (Fase 5)
// vive em auth.go (RequirePangolinIdentity, /v1/app/* e /v1/admin/*) e no pacote
// portal (RequireApproved, gate de aprovação de /v1/portal/*).

// Recover captura um panic no handler e responde 500 no envelope, sem vazar o
// stack ao cliente (detalhe só no log).
func Recover(e *core.RequestEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			e.App.Logger().Error("panic ao atender requisição",
				"request_id", requestID(e),
				"panic", r,
			)
			err = RespondError(e, http.StatusInternalServerError, "erro_interno", "Ocorreu um erro inesperado.", nil)
		}
	}()
	return e.Next()
}

// RequestID gera um id de correlação por requisição, grava no store e o expõe
// no header X-Request-Id (R7.2: o request_id vai no header, não no corpo).
func RequestID(e *core.RequestEvent) error {
	id := ulid.New()
	e.Set(storeRequestID, id)
	e.Response.Header().Set("X-Request-Id", id)
	return e.Next()
}

// Timing grava o instante inicial no store; o elapsed_ms é calculado pelos
// helpers Respond*/Fail ao montar a resposta.
func Timing(e *core.RequestEvent) error {
	e.Set(storeStartedAt, nowFunc())
	return e.Next()
}

// Logging registra o resultado da requisição (método, caminho, status) após o
// handler, correlacionado pelo request_id. Sem conteúdo clínico.
func Logging(e *core.RequestEvent) error {
	err := e.Next()
	e.App.Logger().Info("requisição atendida",
		"request_id", requestID(e),
		"metodo", e.Request.Method,
		"caminho", e.Request.URL.Path,
		"status", e.Status(),
	)
	return err
}

// requestID lê o id de correlação do store (vazio se a chain não rodou).
func requestID(e *core.RequestEvent) string {
	if v, ok := e.Get(storeRequestID).(string); ok {
		return v
	}
	return ""
}
