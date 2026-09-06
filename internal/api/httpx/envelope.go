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
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)
	writeEnvelope(w, r, apiErr.Status, apiErr.Message, nil, map[string]any{
		"code":   apiErr.Code,
		"detail": apiErr.detailForClient(),
	})
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
