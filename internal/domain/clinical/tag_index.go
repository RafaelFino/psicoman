package clinical

import (
	"errors"
	"strings"
)

// TagIndex é uma entrada do índice invertido por paciente/ano. A tag fica em
// CLARO (indexável para busca); o isolamento é por paciente — uma entrada nunca
// cruza pacientes (D4b.2/R6). Entidade nova do MER.
type TagIndex struct {
	ID         string
	PatientID  string
	Ano        int
	Tag        string   // em claro (busca)
	SessionIDs []string // sessões que carregam a tag no ano
}

// Validate verifica as invariantes do índice de tags.
func (t *TagIndex) Validate() error {
	if t.PatientID == "" {
		return errors.New("A tag precisa de um paciente.")
	}
	if t.Ano <= 0 {
		return errors.New("O ano da tag é inválido.")
	}
	if strings.TrimSpace(t.Tag) == "" {
		return errors.New("A tag é obrigatória.")
	}
	return nil
}
