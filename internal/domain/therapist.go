package domain

import (
	"errors"
	"strings"
	"time"
)

// TherapistProfile é o perfil do terapeuta. Há exatamente um por instância
// (requirements §3.9). Foto e arquivos vão para o GED (escopo do terapeuta).
type TherapistProfile struct {
	ID          string
	Name        string
	CRP         string
	Email       string
	Contacts    map[string]string // telefone e outros
	Bio         string
	PhotoFileID string   // ged_file da foto
	LocationIDs []string // locais onde atende
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate verifica as invariantes do perfil.
func (p *TherapistProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("O nome do terapeuta é obrigatório.")
	}
	return nil
}

// PublicView devolve os dados públicos do perfil (exibíveis no portal): nome,
// foto e bio. CRP/contatos/email seguem política de UI e não entram aqui.
func (p *TherapistProfile) PublicView() map[string]any {
	return map[string]any{
		"name":     p.Name,
		"bio":      p.Bio,
		"photo_id": p.PhotoFileID,
	}
}

// TherapistPlatformLink é um link de plataforma de perfil (Doctoralia, etc.).
// Quando corresponde a um canal de aquisição, referencia uma Origin.
type TherapistPlatformLink struct {
	ID        string
	ProfileID string
	Label     string
	URL       string
	OriginID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes do link.
func (l *TherapistPlatformLink) Validate() error {
	if strings.TrimSpace(l.Label) == "" {
		return errors.New("O rótulo do link é obrigatório.")
	}
	if strings.TrimSpace(l.URL) == "" {
		return errors.New("A URL do link é obrigatória.")
	}
	return nil
}
