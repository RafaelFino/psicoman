package clinical

import (
	"errors"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// Anamnesis é a anamnese de um paciente (uma por paciente). O conteúdo clínico
// é SealedText (cifra por campo, D6/R6): nunca string em claro no modelo.
type Anamnesis struct {
	ID        string
	PatientID string
	Content   domain.SealedText
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes da anamnese.
func (a *Anamnesis) Validate() error {
	if a.PatientID == "" {
		return errors.New("A anamnese precisa de um paciente.")
	}
	if a.Content.Empty() {
		return errors.New("O conteúdo da anamnese é obrigatório.")
	}
	return nil
}

// Note é uma nota de prontuário: de sessão (SessionID preenchido) ou livre
// (SessionID vazio). Sempre ordenadas por created_at (requirements §3.6). O
// conteúdo clínico é SealedText (D6/R6).
type Note struct {
	ID        string
	PatientID string
	SessionID string // vazio = nota livre
	Content   domain.SealedText
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsFree indica se é uma nota livre (sem vínculo com sessão).
func (n *Note) IsFree() bool { return n.SessionID == "" }

// Validate verifica as invariantes da nota.
func (n *Note) Validate() error {
	if n.PatientID == "" {
		return errors.New("A nota precisa de um paciente.")
	}
	if n.Content.Empty() {
		return errors.New("O conteúdo da nota é obrigatório.")
	}
	return nil
}
