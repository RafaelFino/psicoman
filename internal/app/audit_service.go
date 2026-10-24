package app

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// AuditService registra operações sensíveis. Falha de auditoria é logada mas
// nunca deve derrubar a operação de negócio (best-effort no ponto de chamada).
// Portado de c3ab585:internal/service/audit.go.
type AuditService struct {
	repo  AuditRepository
	clock clock.Clock
}

// NewAuditService cria o serviço de auditoria.
func NewAuditService(repo AuditRepository) *AuditService {
	return &AuditService{repo: repo, clock: clock.System{}}
}

// Record grava uma entrada de auditoria.
func (s *AuditService) Record(ctx context.Context, actor, action, entity, entityID string, meta map[string]any) error {
	entry := &domain.AuditLog{
		ID:        ulid.New(),
		Actor:     actor,
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		Metadata:  meta,
		CreatedAt: s.clock.Now(),
	}
	return s.repo.Insert(ctx, entry)
}

// List devolve as entradas mais recentes.
func (s *AuditService) List(ctx context.Context, limit int) ([]*domain.AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.repo.List(ctx, limit)
}

// ListByEntity devolve as entradas de uma entidade específica.
func (s *AuditService) ListByEntity(ctx context.Context, entity, entityID string) ([]*domain.AuditLog, error) {
	return s.repo.ListByEntity(ctx, entity, entityID)
}
