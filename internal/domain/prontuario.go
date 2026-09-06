package domain

import (
	"errors"
	"strings"
	"time"
)

// Anamnesis é a anamnese de um paciente (uma por paciente).
type Anamnesis struct {
	ID        string
	PatientID string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Note é uma nota de prontuário: de sessão (SessionID preenchido) ou livre
// (SessionID vazio). Sempre ordenadas por created_at (requirements §3.6).
type Note struct {
	ID        string
	PatientID string
	SessionID string // vazio = nota livre
	Content   string
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
	if strings.TrimSpace(n.Content) == "" {
		return errors.New("O conteúdo da nota é obrigatório.")
	}
	return nil
}

// Template é um modelo em Markdown para enviar ao paciente.
type Template struct {
	ID        string
	Name      string
	BodyMD    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes do template.
func (t *Template) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("O nome do template é obrigatório.")
	}
	if strings.TrimSpace(t.BodyMD) == "" {
		return errors.New("O corpo do template é obrigatório.")
	}
	return nil
}

// TemplateSend é o registro de um envio de template a um paciente, guardando a
// versão renderizada (HTML) — o envio usa sempre a versão formatada.
type TemplateSend struct {
	ID           string
	TemplateID   string
	PatientID    string
	RenderedHTML string
	SentAt       time.Time
}
