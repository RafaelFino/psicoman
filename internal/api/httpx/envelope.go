// Package httpx contém o contrato HTTP compartilhado: envelope de resposta,
// erros de domínio mapeados para HTTP, e helpers de serialização.
//
// Envelope padrão (docs/architecture.md §5):
//
//	{ "message": "texto PT-BR", "elapsed_ms": 12, "data": {}, "error": null }
package httpx

import (
	"encoding/json"
	"net/http"
	"time"
)

// ErrorLogger registra o detalhe técnico de erros 5xx (nunca exposto no corpo).
// Satisfeita por *slog.Logger e pelo logger do projeto. Opcional: sem logger
// configurado, o log é silenciosamente omitido.
type ErrorLogger interface {
	Error(msg string, args ...any)
}

// pkgLogger é o logger de pacote usado por RespondError para registrar o
// detalhe interno dos erros 5xx. Configurado uma vez no boot via SetLogger.
var pkgLogger ErrorLogger

// SetLogger injeta o logger usado para registrar erros internos (5xx). Deve ser
// chamado no boot de cada binário. Passar nil desliga o log de erro.
func SetLogger(l ErrorLogger) { pkgLogger = l }

// ctxKey é o tipo das chaves de contexto deste pacote.
type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyStart
	ctxKeyActor
)

// Envelope é o formato único de toda resposta da API.
type Envelope struct {
	Message   string `json:"message"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Data      any    `json:"data,omitempty"`
	Error     any    `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Respond escreve o envelope com o status informado.
// O elapsed_ms é calculado a partir do início da requisição no contexto.
func Respond(w http.ResponseWriter, r *http.Request, status int, message string, data any) {
	writeEnvelope(w, r, status, message, data, nil)
}

// RespondError escreve um erro no envelope, mapeando APIError para HTTP.
// Erros 5xx têm o detalhe técnico (Internal) registrado no log — nunca no corpo
// da resposta — correlacionado pelo request_id, para diagnóstico.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)
	if apiErr.Status >= http.StatusInternalServerError && pkgLogger != nil {
		detail := apiErr.Internal
		if detail == nil {
			detail = apiErr
		}
		reqID := ""
		if r != nil {
			reqID = RequestID(r.Context())
		}
		pkgLogger.Error("erro interno na requisição",
			"error", detail.Error(),
			"code", apiErr.Code,
			"path", requestPath(r),
			"request_id", reqID,
		)
	}
	writeEnvelope(w, r, apiErr.Status, apiErr.Message, nil, map[string]any{
		"code":   apiErr.Code,
		"detail": apiErr.detailForClient(),
	})
}

// requestPath devolve o caminho da requisição (vazio se r for nil).
func requestPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}

func writeEnvelope(w http.ResponseWriter, r *http.Request, status int, message string, data, errObj any) {
	env := Envelope{
		Message:   message,
		Data:      data,
		Error:     errObj,
		ElapsedMS: elapsedMS(r),
		RequestID: RequestID(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

func elapsedMS(r *http.Request) int64 {
	if r == nil {
		return 0
	}
	if start, ok := r.Context().Value(ctxKeyStart).(time.Time); ok {
		return time.Since(start).Milliseconds()
	}
	return 0
}

// DecodeJSON lê o corpo JSON da requisição em dst, com limite de tamanho.
func DecodeJSON(r *http.Request, dst any) error {
	const maxBody = 5 << 20 // 5 MiB
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrInvalidBody("Não foi possível interpretar os dados enviados.")
	}
	return nil
}
