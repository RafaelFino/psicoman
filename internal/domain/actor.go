package domain

import "context"

// ActorSystem é o ator padrão para operações sem request (jobs, hooks de boot,
// backup). Mantém o valor histórico "sistema" usado nas 11 chamadas de
// AuditService.Record que não têm identidade de terapeuta.
const ActorSystem = "sistema"

// actorKey é a chave opaca do ator no context. Tipo privado para evitar colisão
// com outras chaves de context (padrão recomendado pela stdlib).
type actorKey struct{}

// WithActor devolve um context derivado carregando o ator (e-mail/handle do
// terapeuta resolvido pela identidade do Pangolin). Propagado pelo middleware
// httpx.PropagateActor e lido por AuditService.Record via ActorFrom.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

// ActorFrom extrai o ator do context; devolve "" quando ausente (sem request,
// ou request que não passou pelo middleware de propagação).
func ActorFrom(ctx context.Context) string {
	if v, ok := ctx.Value(actorKey{}).(string); ok {
		return v
	}
	return ""
}
