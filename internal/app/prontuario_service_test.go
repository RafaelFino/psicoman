package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// newProntuario monta o ProntuarioService com o fake repo e a auditoria fake.
func newProntuario(t *testing.T) (*ProntuarioService, *fakeProntuarioRepo, *fakeAuditRepo) {
	t.Helper()
	repo := newProntuarioRepo()
	audit, auditRepo := newAudit()
	svc := NewProntuarioService(repo, nil)
	svc.SetAudit(audit)
	return svc, repo, auditRepo
}

// metaContemTexto varre recursivamente os valores string de um meta à procura
// do claro clínico. Guarda contra regressão que coloque texto no audit_log.
func metaContemTexto(meta map[string]any, texto string) bool {
	for _, v := range meta {
		switch s := v.(type) {
		case string:
			if strings.Contains(s, texto) {
				return true
			}
		case map[string]any:
			if metaContemTexto(s, texto) {
				return true
			}
		case fmt.Stringer:
			if strings.Contains(s.String(), texto) {
				return true
			}
		}
	}
	return false
}

func TestProntuario_AddNote_AuditaCreate(t *testing.T) {
	svc, _, auditRepo := newProntuario(t)
	const clinico = "paciente-relata-crise-de-ansiedade"

	n, err := svc.AddNote(context.Background(), AddNoteInput{PatientID: "p1", SessionID: "s1", Content: clinico})
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	acts := auditRepo.actionsFor("note", n.ID)
	if len(acts) != 1 || acts[0] != domain.AuditActionCreate {
		t.Fatalf("esperava uma ação criar em note, obteve %v", acts)
	}
	for _, e := range auditRepo.entries {
		if metaContemTexto(e.Metadata, clinico) {
			t.Fatalf("meta vazou o texto clínico: %+v", e.Metadata)
		}
	}
}

func TestProntuario_UpsertAnamnesis_CriarEAtualizar(t *testing.T) {
	svc, _, auditRepo := newProntuario(t)
	const clinico = "historia-familiar-sensivel"

	a, err := svc.UpsertAnamnesis(context.Background(), UpsertAnamnesisInput{PatientID: "p1", Content: clinico})
	if err != nil {
		t.Fatalf("UpsertAnamnesis (criar): %v", err)
	}
	// Segundo upsert do mesmo paciente => atualizar.
	if _, err := svc.UpsertAnamnesis(context.Background(), UpsertAnamnesisInput{PatientID: "p1", Content: clinico + "-v2"}); err != nil {
		t.Fatalf("UpsertAnamnesis (atualizar): %v", err)
	}
	acts := auditRepo.actionsFor("anamnesis", a.ID)
	if len(acts) != 2 || acts[0] != domain.AuditActionCreate || acts[1] != domain.AuditActionUpdate {
		t.Fatalf("esperava [criar, atualizar] em anamnesis, obteve %v", acts)
	}
	for _, e := range auditRepo.entries {
		if metaContemTexto(e.Metadata, "sensivel") {
			t.Fatalf("meta vazou o texto clínico: %+v", e.Metadata)
		}
	}
}

func TestProntuario_GetChart_AuditaRead(t *testing.T) {
	svc, _, auditRepo := newProntuario(t)
	const clinico = "nota-clinica-confidencial"
	if _, err := svc.AddNote(context.Background(), AddNoteInput{PatientID: "p1", Content: clinico}); err != nil {
		t.Fatalf("AddNote (setup): %v", err)
	}

	_, _, err := svc.GetChart(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetChart: %v", err)
	}
	acts := auditRepo.actionsFor("prontuario", "p1")
	if len(acts) != 1 || acts[0] != domain.AuditActionRead {
		t.Fatalf("esperava uma ação ler em prontuario, obteve %v", acts)
	}
	// Confere a forma do meta e a ausência do claro.
	for _, e := range auditRepo.entries {
		if e.Entity == "prontuario" {
			if e.Metadata["notas"] != 1 {
				t.Errorf("meta.notas esperado 1, obteve %v", e.Metadata["notas"])
			}
			if e.Metadata["anamnese"] != false {
				t.Errorf("meta.anamnese esperado false, obteve %v", e.Metadata["anamnese"])
			}
		}
		if metaContemTexto(e.Metadata, clinico) {
			t.Fatalf("meta vazou o texto clínico: %+v", e.Metadata)
		}
	}
}

func TestProntuario_FalhaDaOperacaoNaoAudita(t *testing.T) {
	repo := newProntuarioRepo()
	repo.createNoteErr = errors.New("db down")
	audit, auditRepo := newAudit()
	svc := NewProntuarioService(repo, nil)
	svc.SetAudit(audit)

	if _, err := svc.AddNote(context.Background(), AddNoteInput{PatientID: "p1", Content: "x"}); err == nil {
		t.Fatal("esperava erro da operação")
	}
	if len(auditRepo.entries) != 0 {
		t.Fatalf("falha da operação não deve auditar, obteve %d", len(auditRepo.entries))
	}
}

func TestProntuario_AuditNilNaoPanica(t *testing.T) {
	repo := newProntuarioRepo()
	svc := NewProntuarioService(repo, nil) // sem SetAudit

	if _, err := svc.AddNote(context.Background(), AddNoteInput{PatientID: "p1", Content: "x"}); err != nil {
		t.Fatalf("AddNote sem audit: %v", err)
	}
	if _, err := svc.UpsertAnamnesis(context.Background(), UpsertAnamnesisInput{PatientID: "p1", Content: "y"}); err != nil {
		t.Fatalf("UpsertAnamnesis sem audit: %v", err)
	}
	if _, _, err := svc.GetChart(context.Background(), "p1"); err != nil {
		t.Fatalf("GetChart sem audit: %v", err)
	}
}
