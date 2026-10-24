package app

import (
	"context"
	"errors"
	"strings"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// AISummaryInput é o MÍNIMO que o serviço passa à IA: só o texto clínico já em
// claro em memória (minimização — psicoman-seguranca-lgpd.md). Nenhum id de
// paciente/sessão vai para a IA.
type AISummaryInput struct {
	Text string
}

// AISummaryOutput é o retorno da IA: resumo (texto) + tags (claro).
type AISummaryOutput struct {
	Summary string
	Tags    []string
}

// AISummarizer é a porta de IA consumida pelo serviço (satisfeita pelo adapter
// sobre internal/integration/gemini). Desacopla o app do SDK/HTTP do Gemini e
// permite fake nos testes. nil => IA desligada (opt-in pela presença da chave).
type AISummarizer interface {
	Summarize(ctx context.Context, in AISummaryInput) (*AISummaryOutput, error)
}

// AIService orquestra o resumo clínico por IA: disparo EXPLÍCITO pelo terapeuta,
// resultado REVISÁVEL (sugerido → revisado → consolidado) antes de entrar no
// índice de tags, auditoria ai.process SEM conteúdo clínico (minimização) e
// opt-in por instância (default OFF: sem Summarizer, nada roda). O resumo é
// clínico e persiste como SealedText (cifrado na borda pelo repositório).
type AIService struct {
	ai        AISummarizer
	summaries AISummaryRepository
	tags      TagIndexRepository
	sessions  SessionRepository
	audit     *AuditService
	clock     clock.Clock
}

// NewAIService cria o serviço. ai nil => IA desligada (Enabled() == false).
func NewAIService(ai AISummarizer, summaries AISummaryRepository, tags TagIndexRepository, sessions SessionRepository, audit *AuditService) *AIService {
	return &AIService{
		ai:        ai,
		summaries: summaries,
		tags:      tags,
		sessions:  sessions,
		audit:     audit,
		clock:     clock.System{},
	}
}

// Enabled reporta se a IA está ligada (opt-in pela presença da chave → presença
// do Summarizer injetado no wiring). Com IA off, Suggest recusa.
func (s *AIService) Enabled() bool { return s.ai != nil }

// Suggestion é o resultado de um disparo de IA: o resumo persistido (status
// "sugerido", cifrado) e as tags SUGERIDAS (em claro, ainda NÃO indexadas). O
// terapeuta revisa o resumo e as tags e, ao aprovar, chama Consolidate com a
// lista final — é o passo de revisão humana (R10.4).
type Suggestion struct {
	Summary       *clinical.AISummary
	SuggestedTags []string
}

// Suggest dispara a IA para uma sessão: envia o texto clínico (minimizado),
// grava o resumo como "sugerido" (cifrado) e audita ai.process SEM clínico.
// Recusa se a IA estiver desligada. NÃO consolida nem indexa tags ainda — o
// resultado é revisável (Consolidate). As tags sugeridas voltam no Suggestion
// para o terapeuta revisar; só entram no índice ao consolidar.
func (s *AIService) Suggest(ctx context.Context, actor, sessionID, clinicalText string) (*Suggestion, error) {
	if !s.Enabled() {
		return nil, NewValidation("A assistência por IA não está habilitada nesta instância.")
	}
	if _, err := s.sessions.Get(ctx, sessionID); err != nil {
		return nil, err
	}
	out, err := s.ai.Summarize(ctx, AISummaryInput{Text: clinicalText})
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	sum := &clinical.AISummary{
		ID:          ulid.New(),
		SessionID:   sessionID,
		Resumo:      domain.NewSealedText(out.Summary),
		Status:      clinical.AISummarySugerido,
		GeneratedAt: now,
	}
	if err := sum.Validate(); err != nil {
		return nil, NewValidation(err.Error())
	}
	if err := s.summaries.Create(ctx, sum); err != nil {
		return nil, err
	}
	// Auditoria MINIMIZADA: fato do processamento, nunca o texto clínico nem as
	// tags. Guarda só a contagem de tags sugeridas como metadado numérico.
	if s.audit != nil {
		_ = s.audit.Record(ctx, actor, domain.AuditActionAIProcess, "ai_summary", sum.ID,
			map[string]any{"session_id": sessionID, "tags_sugeridas": len(out.Tags)})
	}
	return &Suggestion{Summary: sum, SuggestedTags: normalizeTags(out.Tags)}, nil
}

// Consolidate marca o resumo como consolidado e, então, indexa as tags no
// tag_index (por paciente/ano, em claro). As tags vêm REVISADAS pelo terapeuta
// (o handler passa a lista final), garantindo o passo de revisão humana.
func (s *AIService) Consolidate(ctx context.Context, summaryID string, reviewedTags []string) (*clinical.AISummary, error) {
	if !s.Enabled() {
		return nil, NewValidation("A assistência por IA não está habilitada nesta instância.")
	}
	sum, err := s.summaries.Get(ctx, summaryID)
	if err != nil {
		return nil, err
	}
	sess, err := s.sessions.Get(ctx, sum.SessionID)
	if err != nil {
		return nil, err
	}
	if err := s.summaries.UpdateStatus(ctx, sum.ID, clinical.AISummaryConsolidado); err != nil {
		return nil, err
	}
	sum.Status = clinical.AISummaryConsolidado

	ano := sess.StartsAt.Year()
	for _, tag := range normalizeTags(reviewedTags) {
		if err := s.tags.Upsert(ctx, sess.PatientID, ano, tag, sess.ID); err != nil {
			return nil, err
		}
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "sistema", domain.AuditActionAIProcess, "ai_summary", sum.ID,
			map[string]any{"session_id": sess.ID, "consolidado": true, "tags_indexadas": len(normalizeTags(reviewedTags))})
	}
	return sum, nil
}

// SearchByTag devolve as sessões do paciente (no ano) que carregam a tag. A
// busca é SEMPRE isolada por paciente (R10.2): não há busca cross-paciente.
func (s *AIService) SearchByTag(ctx context.Context, patientID string, ano int, tag string) ([]string, error) {
	tag = normalizeTag(tag)
	if tag == "" {
		return nil, NewValidation("A tag de busca é obrigatória.")
	}
	ids, err := s.tags.FindByTag(ctx, patientID, ano, tag)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []string{}, nil
		}
		return nil, err
	}
	return ids, nil
}

// normalizeTag limpa uma tag (trim + minúsculas).
func normalizeTag(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

// normalizeTags limpa a lista de tags (trim, minúsculas, sem vazias/dup).
// Espelha gemini.NormalizeTags sem acoplar o app ao pacote de integração.
func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = normalizeTag(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
