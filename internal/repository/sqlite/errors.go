package sqlite

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/RafaelFino/psicoman/internal/service"
)

// mapError traduz erros do driver para os erros sentinela de serviço.
// Constraint UNIQUE → ErrConflict; sql.ErrNoRows → ErrNotFound.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique constraint") || strings.Contains(msg, "constraint failed") && strings.Contains(msg, "unique") {
		return service.NewConflict("Registro duplicado.")
	}
	return err
}
