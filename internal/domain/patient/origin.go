package patient

import (
	"errors"
	"strings"
	"time"
)

// Origin é um canal de aquisição de paciente (Doctoralia, indicação, etc.).
type Origin struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes da origem.
func (o *Origin) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return errors.New("O nome da origem é obrigatório.")
	}
	return nil
}
