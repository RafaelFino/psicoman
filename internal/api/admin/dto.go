// Package admin reúne os handlers de operação técnica (/v1/admin/*). Não importa
// os pacotes dos outros atores (segregação por ator, D2).
package admin

import (
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
)

// AuditLogDTO expõe uma entrada de auditoria. Por contrato de domínio, a
// metadata NÃO contém conteúdo clínico. O mapper lê os campos reais de
// domain.AuditLog: CreatedAt→timestamp (via FmtTime) e Metadata→meta.
type AuditLogDTO struct {
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Entity    string         `json:"entity"`
	EntityID  string         `json:"entity_id"`
	Timestamp string         `json:"timestamp"`
	Meta      map[string]any `json:"meta"`
}

func toAuditLogDTO(a *domain.AuditLog) AuditLogDTO {
	return AuditLogDTO{
		Actor:     a.Actor,
		Action:    a.Action,
		Entity:    a.Entity,
		EntityID:  a.EntityID,
		Timestamp: httpx.FmtTime(a.CreatedAt),
		Meta:      a.Metadata,
	}
}

func toAuditLogDTOs(as []*domain.AuditLog) []AuditLogDTO {
	out := make([]AuditLogDTO, 0, len(as))
	for _, a := range as {
		out = append(out, toAuditLogDTO(a))
	}
	return out
}
