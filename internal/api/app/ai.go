package app

import (
	"strconv"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
)

type aiSuggestBody struct {
	// Text é o conteúdo clínico a resumir (disparo explícito do terapeuta). O
	// servidor envia SÓ este texto à IA (minimização — sem id de paciente).
	Text string `json:"text"`
}

type aiConsolidateBody struct {
	// Tags revisadas pelo terapeuta (lista final a indexar).
	Tags []string `json:"tags"`
}

// SuggestAISummary POST /v1/app/sessions/{id}/ai-summary: dispara a IA para a
// sessão (opt-in; 422 se desligada), grava o resumo "sugerido" (cifrado) e
// devolve o resumo + tags sugeridas para revisão. Não indexa tags ainda.
func (h *Handlers) SuggestAISummary(e *core.RequestEvent) error {
	sessionID := e.Request.PathValue("id")
	var body aiSuggestBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	actor := httpx.ActorIdentity(e)
	ctx, cancel := reqCtx(e)
	defer cancel()
	sug, err := h.svc.AI.Suggest(ctx, actor, sessionID, body.Text)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Resumo sugerido pela IA (revise antes de consolidar).", AISuggestionDTO{
		SummaryID:     sug.Summary.ID,
		SessionID:     sug.Summary.SessionID,
		Summary:       sug.Summary.Resumo.Reveal(),
		Status:        sug.Summary.Status,
		SuggestedTags: sug.SuggestedTags,
	})
}

// ConsolidateAISummary POST /v1/app/ai-summaries/{id}/consolidate: marca o
// resumo como consolidado e indexa as tags REVISADAS no índice por paciente/ano.
func (h *Handlers) ConsolidateAISummary(e *core.RequestEvent) error {
	summaryID := e.Request.PathValue("id")
	var body aiConsolidateBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	sum, err := h.svc.AI.Consolidate(ctx, summaryID, body.Tags)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Resumo consolidado e tags indexadas.", AISummaryDTO{
		SummaryID: sum.ID,
		SessionID: sum.SessionID,
		Summary:   sum.Resumo.Reveal(),
		Status:    sum.Status,
	})
}

// SearchByTag GET /v1/app/patients/{id}/tags?tag=&ano=: busca as sessões do
// paciente (no ano) que carregam a tag. Isolado por paciente (R10.2).
func (h *Handlers) SearchByTag(e *core.RequestEvent) error {
	patientID := e.Request.PathValue("id")
	tag := e.Request.URL.Query().Get("tag")
	ano, _ := strconv.Atoi(e.Request.URL.Query().Get("ano"))
	ctx, cancel := reqCtx(e)
	defer cancel()
	ids, err := h.svc.AI.SearchByTag(ctx, patientID, ano, tag)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessões com a tag.", TagSearchDTO{
		Tag:        tag,
		Ano:        ano,
		SessionIDs: ids,
	})
}
