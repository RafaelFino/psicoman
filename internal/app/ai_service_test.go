package app

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func seedSession(repo *fakeSessionRepo, id, patientID string, startsAt time.Time) {
	repo.byID[id] = &scheduling.Session{
		ID: id, PatientID: patientID, Modality: scheduling.ModalityOnline,
		Status: scheduling.SessionRealizada, StartsAt: startsAt, EndsAt: startsAt.Add(time.Hour),
	}
}

func newAISvc(ai AISummarizer) (*AIService, *fakeAISummaryRepo, *fakeTagIndexRepo, *fakeSessionRepo, *fakeAuditRepo) {
	summaries := newAISummaryRepo()
	tags := newTagIndexRepo()
	sessions := newSessionRepo()
	audit, arepo := newAudit()
	svc := NewAIService(ai, summaries, tags, sessions, audit)
	svc.clock = clock.Fixed{T: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location())}
	return svc, summaries, tags, sessions, arepo
}

// TestAI_Desligada_Recusa: sem Summarizer (IA off, opt-in pela chave ausente),
// Suggest recusa com validação e nada é gravado.
func TestAI_Desligada_Recusa(t *testing.T) {
	ctx := context.Background()
	svc, summaries, _, sessions, _ := newAISvc(nil)
	seedSession(sessions, "s1", "p1", time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()))
	if svc.Enabled() {
		t.Fatal("IA sem Summarizer deveria estar desligada")
	}
	if _, err := svc.Suggest(ctx, "terapeuta", "s1", "texto"); err == nil {
		t.Fatal("IA off deveria recusar Suggest")
	}
	if len(summaries.byID) != 0 {
		t.Fatal("IA off não deveria gravar resumo")
	}
}

// TestAI_Suggest_GravaSugeridoEMinimizaAuditoria: Suggest grava o resumo
// "sugerido" (cifrado), devolve as tags para revisão SEM indexá-las, e audita
// ai.process sem conteúdo clínico (minimização).
func TestAI_Suggest_GravaSugeridoEMinimizaAuditoria(t *testing.T) {
	ctx := context.Background()
	ai := &fakeSummarizer{result: AISummaryOutput{Summary: "resumo da sessão", Tags: []string{"ansiedade", "sono"}}}
	svc, summaries, tags, sessions, arepo := newAISvc(ai)
	seedSession(sessions, "s1", "p1", time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()))

	const clinicalText = "paciente relatou crise de ansiedade e insônia"
	sug, err := svc.Suggest(ctx, "terapeuta@x.com", "s1", clinicalText)
	if err != nil {
		t.Fatal(err)
	}
	// Resumo gravado como sugerido, cifrável (SealedText com claro em memória).
	if sug.Summary.Status != clinical.AISummarySugerido {
		t.Fatalf("status inicial deveria ser sugerido, veio %q", sug.Summary.Status)
	}
	if sug.Summary.Resumo.Reveal() != "resumo da sessão" {
		t.Fatalf("resumo não preservado: %q", sug.Summary.Resumo.Reveal())
	}
	if len(summaries.byID) != 1 {
		t.Fatalf("esperava 1 resumo gravado, veio %d", len(summaries.byID))
	}
	// Tags sugeridas voltam para revisão, mas NÃO foram indexadas ainda.
	if len(sug.SuggestedTags) != 2 {
		t.Fatalf("esperava 2 tags sugeridas, veio %v", sug.SuggestedTags)
	}
	if len(tags.entries) != 0 {
		t.Fatal("tags não deveriam ser indexadas antes de consolidar")
	}
	// Minimização do ENVIO: só o texto clínico foi para a IA (sem id/paciente).
	if len(ai.calls) != 1 || ai.calls[0].Text != clinicalText {
		t.Fatalf("IA deveria receber só o texto clínico: %+v", ai.calls)
	}
	// Auditoria ai.process SEM conteúdo clínico: nada do texto nem das tags.
	entry := arepo.entries[len(arepo.entries)-1]
	if entry.Action != domain.AuditActionAIProcess {
		t.Fatalf("ação de auditoria inesperada: %q", entry.Action)
	}
	for k, v := range entry.Metadata {
		if s, ok := v.(string); ok && (s == clinicalText || s == "resumo da sessão" || s == "ansiedade" || s == "sono") {
			t.Fatalf("auditoria vazou conteúdo clínico em %q: %v", k, v)
		}
	}
}

// TestAI_Consolidate_IndexaTags: ao consolidar, o resumo vira consolidado e as
// tags revisadas entram no índice por paciente/ano.
func TestAI_Consolidate_IndexaTags(t *testing.T) {
	ctx := context.Background()
	ai := &fakeSummarizer{result: AISummaryOutput{Summary: "r", Tags: []string{"ansiedade"}}}
	svc, summaries, tags, sessions, _ := newAISvc(ai)
	start := time.Date(2026, 5, 10, 10, 0, 0, 0, clock.Location())
	seedSession(sessions, "s1", "p1", start)

	sug, err := svc.Suggest(ctx, "t", "s1", "texto")
	if err != nil {
		t.Fatal(err)
	}
	// Terapeuta revisa e consolida com tags finais (inclui uma nova).
	sum, err := svc.Consolidate(ctx, sug.Summary.ID, []string{"Ansiedade", "sono"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != clinical.AISummaryConsolidado {
		t.Fatalf("status deveria ser consolidado, veio %q", sum.Status)
	}
	if summaries.byID[sum.ID].Status != clinical.AISummaryConsolidado {
		t.Fatal("status consolidado não persistido")
	}
	// Tags indexadas no ano da sessão (2026), normalizadas, por paciente.
	ids, err := svc.SearchByTag(ctx, "p1", 2026, "ansiedade")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("busca por tag deveria achar a sessão: %v", ids)
	}
	if _, ok := tags.entries[tagKey("p1", 2026, "sono")]; !ok {
		t.Fatal("tag 'sono' deveria ter sido indexada")
	}
}

// TestAI_SearchByTag_IsoladoPorPaciente: a busca por tag nunca cruza pacientes
// (R10.2): a mesma tag num paciente não aparece para outro.
func TestAI_SearchByTag_IsoladoPorPaciente(t *testing.T) {
	ctx := context.Background()
	ai := &fakeSummarizer{result: AISummaryOutput{Summary: "r", Tags: []string{"ansiedade"}}}
	svc, _, _, sessions, _ := newAISvc(ai)
	start := time.Date(2026, 5, 10, 10, 0, 0, 0, clock.Location())
	seedSession(sessions, "s1", "p1", start)

	sug, _ := svc.Suggest(ctx, "t", "s1", "texto")
	if _, err := svc.Consolidate(ctx, sug.Summary.ID, []string{"ansiedade"}); err != nil {
		t.Fatal(err)
	}
	// Outro paciente (p2) não vê a tag de p1.
	ids, err := svc.SearchByTag(ctx, "p2", 2026, "ansiedade")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("busca de p2 não deveria achar sessões de p1 (isolamento): %v", ids)
	}
}
