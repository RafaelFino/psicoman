package app

import (
	"context"
	"errors"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
)

func TestAuditRecord_AtorDoContexto(t *testing.T) {
	audit, repo := newAudit()
	ctx := domain.WithActor(context.Background(), "terapeuta@x")

	// actor "" => resolve do contexto.
	if err := audit.Record(ctx, "", domain.AuditActionRead, "prontuario", "p1", nil); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(repo.entries) != 1 || repo.entries[0].Actor != "terapeuta@x" {
		t.Fatalf("esperava ator do contexto, obteve %+v", repo.entries)
	}
}

func TestAuditRecord_SistemaExplicitoTrocaPeloContexto(t *testing.T) {
	audit, repo := newAudit()
	ctx := domain.WithActor(context.Background(), "terapeuta@x")

	// actor "sistema" explícito, com ator no contexto => troca pelo ator real.
	if err := audit.Record(ctx, domain.ActorSystem, domain.AuditActionUpdate, "anamnesis", "a1", nil); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if repo.entries[0].Actor != "terapeuta@x" {
		t.Fatalf("esperava troca por terapeuta@x, obteve %q", repo.entries[0].Actor)
	}
}

func TestAuditRecord_SemAtorGravaSistema(t *testing.T) {
	audit, repo := newAudit()

	// Sem ator no contexto e sem actor => "sistema".
	if err := audit.Record(context.Background(), "", domain.AuditActionBackup, "backup", "", nil); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if repo.entries[0].Actor != domain.ActorSystem {
		t.Fatalf("esperava %q, obteve %q", domain.ActorSystem, repo.entries[0].Actor)
	}
}

func TestAuditRecord_FalhaDoRepoNaoGravaEhObservavel(t *testing.T) {
	audit, repo := newAudit()
	repo.insertErr = errors.New("db indisponível")

	err := audit.Record(context.Background(), "a@x", domain.AuditActionRead, "prontuario", "p1", nil)
	if err == nil {
		t.Fatal("esperava erro propagado do Insert (o caller usa _ =, mas Record devolve)")
	}
	if len(repo.entries) != 0 {
		t.Fatalf("nada deveria ter sido gravado, obteve %d", len(repo.entries))
	}
	// O log da falha acontece dentro de Record; aqui garantimos que não panica
	// e que o default logger não é nil (SetLogger não foi chamado).
}
