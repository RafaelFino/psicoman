package httpx

import (
	"errors"
	"net/http"
)

// APIError é um erro com código HTTP e mensagem PT-BR já pronta para o cliente.
// As mensagens internas (Internal) nunca vazam para o corpo da resposta.
type APIError struct {
	Status   int
	Code     string
	Message  string // PT-BR, exibível ao usuário
	Internal error  // detalhe técnico (log apenas)
}

// Error implementa error.
func (e *APIError) Error() string {
	if e.Internal != nil {
		return e.Code + ": " + e.Internal.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap expõe o erro interno.
func (e *APIError) Unwrap() error { return e.Internal }

// detailForClient devolve detalhe seguro para o cliente (nunca o erro interno).
func (e *APIError) detailForClient() any { return nil }

// WithInternal anexa um erro técnico para log, preservando a mensagem PT-BR.
func (e *APIError) WithInternal(err error) *APIError {
	cp := *e
	cp.Internal = err
	return &cp
}

// AsAPIError converte qualquer erro em APIError; erros desconhecidos viram 500.
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return &APIError{
		Status:   http.StatusInternalServerError,
		Code:     "erro_interno",
		Message:  "Ocorreu um erro interno. Tente novamente em instantes.",
		Internal: err,
	}
}

// Construtores de erros comuns (mensagens em PT-BR).

// ErrBadRequest — 400.
func ErrBadRequest(msg string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "requisicao_invalida", Message: msg}
}

// ErrInvalidBody — 400 para corpo malformado.
func ErrInvalidBody(msg string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "corpo_invalido", Message: msg}
}

// ErrValidation — 422 para regra de validação de negócio.
func ErrValidation(msg string) *APIError {
	return &APIError{Status: http.StatusUnprocessableEntity, Code: "validacao", Message: msg}
}

// ErrUnauthorized — 401.
func ErrUnauthorized(msg string) *APIError {
	return &APIError{Status: http.StatusUnauthorized, Code: "nao_autenticado", Message: msg}
}

// ErrForbidden — 403.
func ErrForbidden(msg string) *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "acesso_negado", Message: msg}
}

// ErrNotFound — 404.
func ErrNotFound(msg string) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "nao_encontrado", Message: msg}
}

// ErrConflict — 409 (ex: conflito de agenda, unicidade).
func ErrConflict(msg string) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "conflito", Message: msg}
}

// ErrTooManyRequests — 429 (rate limit).
func ErrTooManyRequests(msg string) *APIError {
	return &APIError{Status: http.StatusTooManyRequests, Code: "muitas_requisicoes", Message: msg}
}

// ErrInternal — 500 com detalhe técnico só no log.
func ErrInternal(internal error) *APIError {
	return &APIError{
		Status:   http.StatusInternalServerError,
		Code:     "erro_interno",
		Message:  "Ocorreu um erro interno. Tente novamente em instantes.",
		Internal: internal,
	}
}
