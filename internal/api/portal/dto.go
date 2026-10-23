// Package portal reúne os handlers do ATOR paciente (/v1/portal/*). Regra
// inviolável (D6.3/R6.3): NENHUM DTO do portal tem campo clínico — este pacote
// não importa o bounded context clínico e nunca revela texto clínico. Também
// não importa os pacotes dos outros atores (segregação por ator, D2). O teste
// estático em leak_test.go reforça essa invariante.
package portal

import (
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// PortalMeDTO é o que o paciente vê de si. SEM qualquer campo clínico.
type PortalMeDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	ApprovalStatus string `json:"approval_status"`
}

// PortalSessionDTO é a agenda do paciente — horário/status, nunca nota/anamnese.
type PortalSessionDTO struct {
	ID       string `json:"id"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Status   string `json:"status"`
	Modality string `json:"modality"`
	MeetURL  string `json:"meet_url,omitempty"`
}

func toMeDTO(p *patient.Patient) PortalMeDTO {
	return PortalMeDTO{
		ID:             p.ID,
		Name:           p.Name,
		Email:          p.Email,
		Phone:          p.Phone,
		ApprovalStatus: p.ApprovalStatus,
	}
}

func toSessionDTO(s *scheduling.Session) PortalSessionDTO {
	return PortalSessionDTO{
		ID:       s.ID,
		StartsAt: httpx.FmtTime(s.StartsAt),
		EndsAt:   httpx.FmtTime(s.EndsAt),
		Status:   s.Status,
		Modality: s.Modality,
		MeetURL:  s.MeetURL,
	}
}

func toSessionDTOs(ss []*scheduling.Session) []PortalSessionDTO {
	out := make([]PortalSessionDTO, 0, len(ss))
	for _, s := range ss {
		out = append(out, toSessionDTO(s))
	}
	return out
}
