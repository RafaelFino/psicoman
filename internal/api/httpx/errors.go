package httpx

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
)

// messageMaxLen limita o tamanho da message de erro escrita no corpo. Guarda
// defensiva (finding-5): a message de um 4xx é sempre uma frase PT-BR curta do
// serviço; a truncagem evita que uma mensagem longa inesperada vaze volume.
const messageMaxLen = 300

// MapError traduz um erro de app para (status, code PT-BR, message PT-BR) via
// errors.Is. Erros não mapeados viram 500 com mensagem fixa — o detalhe técnico
// NUNCA entra no corpo, só no log (ver RespondError/Fail).
func MapError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, app.ErrNotFound):
		return http.StatusNotFound, "nao_encontrado", "Registro não encontrado."
	case errors.Is(err, app.ErrConflict):
		return http.StatusConflict, "conflito", err.Error()
	case errors.Is(err, app.ErrValidation):
		return http.StatusUnprocessableEntity, "validacao", err.Error()
	default:
		return http.StatusInternalServerError, "erro_interno", "Ocorreu um erro inesperado."
	}
}

// RespondError responde status com error.code e message; internalErr vai SÓ
// para o log (nunca ao corpo), correlacionado pelo request_id.
func RespondError(e *core.RequestEvent, status int, code, message string, internalErr error) error {
	if internalErr != nil {
		e.App.Logger().Error("erro ao atender requisição",
			"request_id", requestID(e),
			"status", status,
			"code", code,
			"erro", internalErr.Error(),
		)
	}
	return e.JSON(status, Envelope{
		Message:   truncate(message, messageMaxLen),
		ElapsedMS: elapsedMS(e),
		Error:     &APIError{Code: code},
	})
}

// Fail é o caminho padrão dos handlers para erros de serviço: mapeia o erro
// para (status, code, message) e responde. Erros 500 logam o detalhe.
func Fail(e *core.RequestEvent, err error) error {
	status, code, message := MapError(err)
	var internal error
	if status == http.StatusInternalServerError {
		internal = err // detalhe só no log
	}
	return RespondError(e, status, code, message, internal)
}

// FailBadRequest responde 400 requisicao_invalida para corpo malformado. O erro
// bruto do BindBody NÃO vai ao corpo (opcionalmente ao log em debug). É chamado
// explicitamente antes de Fail, pois o erro de BindBody não satisfaz
// errors.Is(err, ErrValidation) e cairia no ramo 500 (correção M3).
func FailBadRequest(e *core.RequestEvent, internalErr error) error {
	if internalErr != nil {
		e.App.Logger().Debug("corpo de requisição inválido",
			"request_id", requestID(e),
			"erro", internalErr.Error(),
		)
	}
	return e.JSON(http.StatusBadRequest, Envelope{
		Message:   "Corpo da requisição inválido.",
		ElapsedMS: elapsedMS(e),
		Error:     &APIError{Code: "requisicao_invalida"},
	})
}

// truncate corta s em no máximo n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
