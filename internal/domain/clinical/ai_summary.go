package clinical

import (
	"errors"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// Status do resumo de IA (revisável antes de consolidar — D4b.2).
const (
	AISummarySugerido    = "sugerido"
	AISummaryRevisado    = "revisado"
	AISummaryConsolidado = "consolidado"
)

// ValidAISummaryStatus indica se s é um status de resumo conhecido.
func ValidAISummaryStatus(s string) bool {
	switch s {
	case AISummarySugerido, AISummaryRevisado, AISummaryConsolidado:
		return true
	default:
		return false
	}
}

// AISummary é o resultado da IA para uma sessão: o corpo do resumo é clínico e,
// portanto, SealedText (D6/R6). Entidade nova do MER (D4b.2).
type AISummary struct {
	ID          string
	SessionID   string
	Resumo      domain.SealedText
	Status      string
	GeneratedAt time.Time
}

// Validate verifica as invariantes do resumo.
func (a *AISummary) Validate() error {
	if a.SessionID == "" {
		return errors.New("O resumo precisa de uma sessão.")
	}
	if a.Resumo.Empty() {
		return errors.New("O conteúdo do resumo é obrigatório.")
	}
	if !ValidAISummaryStatus(a.Status) {
		return errors.New("O estado do resumo é inválido.")
	}
	return nil
}
