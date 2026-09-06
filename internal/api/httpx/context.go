package httpx

import "context"

// WithRequestID guarda o request-id no contexto.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestID recupera o request-id do contexto (vazio se ausente).
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// CtxKeyStart expõe a chave de início da requisição para o middleware de timing.
func CtxKeyStart() any { return ctxKeyStart }

// WithActor guarda o ator autenticado (email) no contexto.
func WithActor(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, ctxKeyActor, email)
}

// Actor recupera o email do ator autenticado (vazio se ausente).
func Actor(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyActor).(string); ok {
		return v
	}
	return ""
}
