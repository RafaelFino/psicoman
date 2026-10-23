// Package httpx reúne o que é transversal ao contrato /v1/* do Psicoman: o
// envelope de resposta, o mapeamento erro→HTTP, a middleware chain e helpers de
// formatação. Não tem regra de negócio e é reusado pelos três atores
// (app/portal/admin), sem que nenhum pacote de ator importe o outro (D2/D7).
package httpx

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Envelope é a resposta padrão de toda rota /v1/*. A ordem das chaves
// {message, elapsed_ms, data, error} é contrato (R7.2) e fica garantida por
// construção: encoding/json serializa campos de struct na ordem de declaração.
// Os helpers Respond*/Fail/FailBadRequest montam SEMPRE esta struct, nunca um
// map[string]any (que seria serializado em ordem alfabética).
type Envelope struct {
	Message   string    `json:"message"`
	ElapsedMS int64     `json:"elapsed_ms"`
	Data      any       `json:"data"`
	Error     *APIError `json:"error"`
}

// APIError é o corpo de erro: code em PT-BR snake_case; nunca detalhe interno.
type APIError struct {
	Code string `json:"code"`
}

// elapsedMS lê o instante inicial gravado pela middleware Timing no store do
// RequestEvent e devolve o tempo decorrido em milissegundos. Sem o marcador
// (ex.: handler fora da chain), devolve 0.
func elapsedMS(e *core.RequestEvent) int64 {
	v := e.Get(storeStartedAt)
	started, ok := v.(time.Time)
	if !ok || started.IsZero() {
		return 0
	}
	ms := time.Since(started).Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return ms
}

// RespondOK responde 200 com data preenchido e error nulo.
func RespondOK(e *core.RequestEvent, message string, data any) error {
	return e.JSON(http.StatusOK, Envelope{Message: message, ElapsedMS: elapsedMS(e), Data: data})
}

// RespondCreated responde 201 (criação de recurso).
func RespondCreated(e *core.RequestEvent, message string, data any) error {
	return e.JSON(http.StatusCreated, Envelope{Message: message, ElapsedMS: elapsedMS(e), Data: data})
}

// RespondAccepted responde 202 (operação aceita para processamento posterior).
func RespondAccepted(e *core.RequestEvent, message string, data any) error {
	return e.JSON(http.StatusAccepted, Envelope{Message: message, ElapsedMS: elapsedMS(e), Data: data})
}
