package portal

import (
	"errors"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/service"
)

// respondServiceError traduz erros de serviço para o envelope HTTP.
func respondServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		httpx.RespondError(w, r, httpx.ErrNotFound("Registro não encontrado."))
	case errors.Is(err, service.ErrConflict):
		httpx.RespondError(w, r, httpx.ErrConflict(err.Error()))
	case errors.Is(err, service.ErrValidation):
		httpx.RespondError(w, r, httpx.ErrValidation(err.Error()))
	default:
		httpx.RespondError(w, r, httpx.ErrInternal(err))
	}
}
