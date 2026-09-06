// Package service contém a inteligência de negócio (orquestração). Depende de
// domain e de interfaces de repositório/integração, permitindo fakes em teste.
package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// AuditRepository persiste registros de auditoria.
type AuditRepository interface {
	Insert(ctx context.Context, entry *domain.AuditLog) error
	List(ctx context.Context, limit int) ([]*domain.AuditLog, error)
}

// AuditService registra operações sensíveis. Falha de auditoria é logada mas
// nunca deve derrubar a operação de negócio (best-effort no ponto de chamada).
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
