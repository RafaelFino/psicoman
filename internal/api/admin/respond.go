package admin

import (
	"errors"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/service"
)

// respondServiceError traduz erros da camada de serviço para o envelope HTTP:
// ErrNotFound→404, ErrConflict→409, ErrValidation→422, resto→500.
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
