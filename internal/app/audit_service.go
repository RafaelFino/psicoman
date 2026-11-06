package app

import (
	"context"
	"log/slog"

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
	log   *slog.Logger
}

// NewAuditService cria o serviço de auditoria. O logger default é
// slog.Default(); o wiring injeta o logger do PocketBase via SetLogger para que
// a falha de gravação caia no mesmo destino dos demais logs do app.
func NewAuditService(repo AuditRepository) *AuditService {
	return &AuditService{repo: repo, clock: clock.System{}, log: slog.Default()}
}

// SetLogger injeta o logger usado para registrar falhas de gravação do
// audit_log. Nil é ignorado (mantém o default).
func (s *AuditService) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// Record grava uma entrada de auditoria. Resolve o ator efetivo a partir do
// context quando o actor recebido é "" ou domain.ActorSystem: assim as chamadas
// que hoje passam "sistema" literal registram a identidade real (terapeuta) nas
// operações disparadas por um request que passou por httpx.PropagateActor, sem
// precisar editar cada ponto de chamada. Ações sem request continuam "sistema".
//
// Best-effort e observável: uma falha de Insert é logada (sem meta, sem ids
// sensíveis) e NÃO é propagada — a trilha pode ter uma lacuna, mas a operação
// de negócio nunca é derrubada por um defeito do audit_log. A contagem da
// métrica audit_log_falhas_total é ligada na Fase 8 (SetMetrics).
func (s *AuditService) Record(ctx context.Context, actor, action, entity, entityID string, meta map[string]any) error {
	if actor == "" || actor == domain.ActorSystem {
		if from := domain.ActorFrom(ctx); from != "" {
			actor = from
		}
	}
	if actor == "" {
		actor = domain.ActorSystem
	}
	entry := &domain.AuditLog{
		ID:        ulid.New(),
		Actor:     actor,
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		Metadata:  meta,
		CreatedAt: s.clock.Now(),
	}
	if err := s.repo.Insert(ctx, entry); err != nil {
		// Sem meta nem conteúdo: só o fato de a trilha ter falhado.
		s.log.Error("falha ao gravar audit_log", "acao", action, "entidade", entity)
		return err
	}
	return nil
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
