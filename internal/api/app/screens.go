package app

import (
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// parseOptionalRange lê um par de datas (date-only AAAA-MM-DD) para um intervalo
// meio-aberto [from, to). Vazio → instante zero (sem limite). O limite final é
// EXCLUSIVO no dia seguinte (o dia 'to' inteiro entra no recorte, consistente
// com os serviços que filtram por [from, to)).
func parseOptionalRange(fromStr, toStr string) (from, to time.Time, err error) {
	if fromStr != "" {
		from, err = time.ParseInLocation("2006-01-02", fromStr, clock.Location())
		if err != nil {
			return
		}
	}
	if toStr != "" {
		to, err = time.ParseInLocation("2006-01-02", toStr, clock.Location())
		if err != nil {
			return
		}
		to = to.Add(24 * time.Hour) // [from, to) com o dia 'to' inteiro incluído
	}
	return
}

// toSessionDTOsFromScheduling projeta uma lista de sessões em DTOs.
func toSessionDTOsFromScheduling(ss []*scheduling.Session) []SessionDTO {
	out := make([]SessionDTO, 0, len(ss))
	for _, s := range ss {
		out = append(out, toSessionDTO(s))
	}
	return out
}

// DayFeed GET /v1/app/day-feed?window= — feed do dia do terapeuta (R1.1).
// window: hoje|7d|recentes (default no serviço). Sem conteúdo clínico; a leitura
// é auditada (AuditActionRead) pelo serviço.
func (h *Handlers) DayFeed(e *core.RequestEvent) error {
	window := e.Request.URL.Query().Get("window")
	ctx, cancel := reqCtx(e)
	defer cancel()
	feed, err := h.svc.DayFeed.Build(ctx, window)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Feed do dia.", feed)
}

// PatientHistory GET /v1/app/patients/{id}/history — história cronológica do
// paciente (R1.2). Composta no servidor; os itens carregam só rótulos legíveis,
// NUNCA conteúdo clínico (SealedText não é revelado). Leitura auditada.
func (h *Handlers) PatientHistory(e *core.RequestEvent) error {
	pid := e.Request.PathValue("id")
	ctx, cancel := reqCtx(e)
	defer cancel()
	items, err := h.svc.History.PatientHistory(ctx, pid)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "História do paciente.", items)
}

// PatientSessionMetrics GET /v1/app/patients/{id}/session-metrics?de=&ate= —
// comparecimento/cancelamento/quebra por categoria (R1.3). Recorte opcional por
// período (de/ate em AAAA-MM-DD); ausentes = histórico completo.
func (h *Handlers) PatientSessionMetrics(e *core.RequestEvent) error {
	pid := e.Request.PathValue("id")
	q := e.Request.URL.Query()
	from, to, perr := parseOptionalRange(q.Get("de"), q.Get("ate"))
	if perr != nil {
		return httpx.FailBadRequest(e, perr)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	m, err := h.svc.Metrics.PatientSessionMetrics(ctx, pid, from, to)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Métricas de sessão do paciente.", m)
}

// ListSessions GET /v1/app/sessions?patient_id=&from=&to= — base do calendário
// mensal (R1.4). Com patient_id, lista as sessões do paciente (opcionalmente
// recortadas por [from,to)); sem patient_id, lista por intervalo [from,to).
func (h *Handlers) ListSessions(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	patientID := q.Get("patient_id")
	from, to, perr := parseOptionalRange(q.Get("from"), q.Get("to"))
	if perr != nil {
		return httpx.FailBadRequest(e, perr)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()

	if patientID != "" {
		ss, err := h.svc.Sessions.ListByPatient(ctx, patientID)
		if err != nil {
			return httpx.Fail(e, err)
		}
		// Recorte opcional por [from,to) no handler (o serviço lista tudo do paciente).
		out := make([]SessionDTO, 0, len(ss))
		for _, s := range ss {
			if !from.IsZero() && s.StartsAt.Before(from) {
				continue
			}
			if !to.IsZero() && !s.StartsAt.Before(to) {
				continue
			}
			out = append(out, toSessionDTO(s))
		}
		return httpx.RespondOK(e, "Sessões do paciente.", out)
	}

	// Sem paciente: exige um intervalo para não varrer tudo.
	if from.IsZero() || to.IsZero() {
		return httpx.RespondError(e, 422, "validacao",
			"Informe patient_id, ou from e to para o intervalo.", nil)
	}
	ss, err := h.svc.Sessions.ListByRange(ctx, from, to)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Sessões no intervalo.", toSessionDTOsFromScheduling(ss))
}
