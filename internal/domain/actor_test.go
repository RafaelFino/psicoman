package domain

import (
	"context"
	"testing"
)

func TestActorFrom_Ausente(t *testing.T) {
	if got := ActorFrom(context.Background()); got != "" {
		t.Fatalf("ctx sem ator: esperava \"\", obteve %q", got)
	}
}

func TestWithActor_ESobrescrita(t *testing.T) {
	ctx := WithActor(context.Background(), "terapeuta@x")
	if got := ActorFrom(ctx); got != "terapeuta@x" {
		t.Fatalf("ctx com ator: esperava %q, obteve %q", "terapeuta@x", got)
	}
	// Sobrescrita: um WithActor posterior vence.
	ctx2 := WithActor(ctx, "outro@x")
	if got := ActorFrom(ctx2); got != "outro@x" {
		t.Fatalf("sobrescrita: esperava %q, obteve %q", "outro@x", got)
	}
	// O context original não é mutado.
	if got := ActorFrom(ctx); got != "terapeuta@x" {
		t.Fatalf("ctx original mutado: esperava %q, obteve %q", "terapeuta@x", got)
	}
}

func TestWithActor_ValorVazioEhPropagado(t *testing.T) {
	// Um ator "" explícito é um valor presente (distinto de ausente); ActorFrom
	// o devolve como "". O caller (AuditService) trata "" como "sem ator".
	ctx := WithActor(context.Background(), "")
	if got := ActorFrom(ctx); got != "" {
		t.Fatalf("ator vazio: esperava \"\", obteve %q", got)
	}
}
