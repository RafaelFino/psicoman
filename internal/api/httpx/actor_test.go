package httpx

import (
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// TestPropagateActor_ColocaAtorNoContexto: com a identidade resolvida no store
// (como faria RequirePangolinIdentity), PropagateActor a injeta no context do
// request, de onde domain.ActorFrom a lê.
func TestPropagateActor_ColocaAtorNoContexto(t *testing.T) {
	e, _ := newEvent(t)
	e.Set(storeActorIdentity, "terapeuta@x")

	if err := PropagateActor(e); err != nil {
		t.Fatalf("PropagateActor: %v", err)
	}
	if got := domain.ActorFrom(e.Request.Context()); got != "terapeuta@x" {
		t.Fatalf("esperava ator no context, obteve %q", got)
	}
}

// TestPropagateActor_SemIdentidadeNaoInjeta: sem identidade no store, o context
// segue sem ator (ActorFrom devolve ""); o audit cairá em "sistema".
func TestPropagateActor_SemIdentidadeNaoInjeta(t *testing.T) {
	e, _ := newEvent(t)

	if err := PropagateActor(e); err != nil {
		t.Fatalf("PropagateActor: %v", err)
	}
	if got := domain.ActorFrom(e.Request.Context()); got != "" {
		t.Fatalf("sem identidade: esperava \"\", obteve %q", got)
	}
}
