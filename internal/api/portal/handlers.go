package portal

import (
	"net/http"
	"time"

	"github.com/RafaelFino/psicoman/internal/api"
	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/service"
)

// parseTime interpreta um horário ISO-8601 (RFC3339).
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// Handlers agrupa os handlers do portal do paciente.
type Handlers struct {
	patients  *service.PatientService
	sessions  *service.PortalSessionService
	locations *service.LocationService
	appts     *service.AppointmentService
	sess      *SessionManager
	verifier  IdentityVerifier
	limiter   *RateLimiter
}

// NewHandlers cria os handlers do portal.
func NewHandlers(patients *service.PatientService, sessions *service.PortalSessionService, locations *service.LocationService, appts *service.AppointmentService, sess *SessionManager, verifier IdentityVerifier, limiter *RateLimiter) *Handlers {
	return &Handlers{patients: patients, sessions: sessions, locations: locations, appts: appts, sess: sess, verifier: verifier, limiter: limiter}
}

// RegisterPublic instala as rotas públicas (sem sessão prévia), com rate limit.
func (h *Handlers) RegisterPublic(g *api.Group) {
	g.Handle("POST", "/login", h.login)
	g.Handle("POST", "/logout", h.logout)
	g.Handle("PUT", "/register", h.register)
}

// RegisterAuthenticated instala as rotas que exigem sessão do paciente.
//
// Dois grupos (defense in depth — R1.2):
//   - authed: só exige sessão válida. Rotas de status/leitura do próprio
//     cadastro, disponíveis mesmo com o cadastro pendente/rejeitado.
//   - gated: exige sessão + aprovação do terapeuta (ApprovalGate). Todas as
//     rotas de recurso (agenda, pedidos, sessões, débitos, edição de perfil).
func (h *Handlers) RegisterAuthenticated(authed, gated *api.Group) {
	// Sempre liberadas (só exigem sessão).
	authed.Handle("GET", "/me", h.me)
	authed.Handle("GET", "/approval-status", h.approvalStatus)

	// Exigem aprovação.
	gated.Handle("PUT", "/me", h.updateMe)
	gated.Handle("GET", "/availability", h.availability)
	gated.Handle("POST", "/appointment-requests", h.requestAppointment)
	gated.Handle("GET", "/appointment-requests", h.myRequests)
	gated.Handle("GET", "/sessions", h.mySessions)
	gated.Handle("GET", "/debts", h.myDebts)
}

// approvalStatus devolve o estado de aprovação do paciente autenticado, usado
// pela UI para decidir o que exibir (área normal, "em análise" ou "não
// liberado"). Disponível mesmo para pendentes/rejeitados (R1.2, R1.3).
func (h *Handlers) approvalStatus(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	p, err := h.patients.GetByEmail(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Estado do seu cadastro.", map[string]any{
		"status": p.ApprovalStatus,
		"name":   p.Name,
		"email":  p.Email,
	})
}

// availability lista as lacunas abertas (locais + janelas de disponibilidade).
func (h *Handlers) availability(w http.ResponseWriter, r *http.Request) {
	locs, err := h.locations.List(r.Context())
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(locs))
	for _, l := range locs {
		windows, err := h.locations.ListAvailability(r.Context(), l.ID)
		if err != nil {
			respondServiceError(w, r, err)
			return
		}
		slots := make([]map[string]any, 0, len(windows))
		for _, wnd := range windows {
			slots = append(slots, map[string]any{
				"weekday":    wnd.Weekday,
				"start_time": wnd.StartTime,
				"end_time":   wnd.EndTime,
			})
		}
		out = append(out, map[string]any{
			"location_id":   l.ID,
			"location_name": l.Name,
			"modality":      l.Modality,
			"slots":         slots,
		})
	}
	httpx.Respond(w, r, http.StatusOK, "Agenda aberta.", out)
}

type requestBody struct {
	LocationID string `json:"location_id"`
	SlotStart  string `json:"slot_start"`
	SlotEnd    string `json:"slot_end"`
	Note       string `json:"note"`
}

// requestAppointment cria um pedido de agendamento (rate limit por email).
func (h *Handlers) requestAppointment(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	if !h.limiter.AllowEmail(email) {
		httpx.RespondError(w, r, httpx.ErrTooManyRequests("Muitos pedidos. Aguarde um instante."))
		return
	}
	var b requestBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	patient, err := h.patients.GetByEmail(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	start, err1 := parseTime(b.SlotStart)
	end, err2 := parseTime(b.SlotEnd)
	if err1 != nil || err2 != nil {
		httpx.RespondError(w, r, httpx.ErrValidation("Os horários devem estar em ISO-8601."))
		return
	}
	req, err := h.appts.Request(r.Context(), service.RequestInput{
		PatientID: patient.ID, LocationID: b.LocationID, SlotStart: start, SlotEnd: end, Note: b.Note,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusCreated, "Pedido de agendamento enviado.", map[string]any{
		"id":     req.ID,
		"status": req.Status,
	})
}

// myRequests devolve os pedidos do paciente autenticado.
func (h *Handlers) myRequests(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	patient, err := h.patients.GetByEmail(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	rs, err := h.appts.ListByPatient(r.Context(), patient.ID)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(rs))
	for _, a := range rs {
		out = append(out, map[string]any{
			"id": a.ID, "slot_start": a.SlotStart, "slot_end": a.SlotEnd, "status": a.Status,
		})
	}
	httpx.Respond(w, r, http.StatusOK, "Seus pedidos.", out)
}

type loginBody struct {
	Credential string `json:"credential"`
}

// login valida a credencial do login social e emite a sessão do portal.
func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var b loginBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	email, err := h.verifier.Verify(r.Context(), b.Credential)
	if err != nil || email == "" {
		httpx.RespondError(w, r, httpx.ErrUnauthorized("Não foi possível validar seu login."))
		return
	}
	h.sess.Issue(w, email)
	httpx.Respond(w, r, http.StatusOK, "Login realizado.", map[string]any{"email": email})
}

// logout limpa a sessão.
func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) {
	h.sess.Clear(w)
	httpx.Respond(w, r, http.StatusOK, "Sessão encerrada.", nil)
}

type registerBody struct {
	Credential string `json:"credential"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	CPF        string `json:"cpf"`
}

// register faz o cadastro básico do paciente (rota pública, com rate limit por
// IP e por email). Vincula por email se já existir (upsert, não duplica).
func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var b registerBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	email, err := h.verifier.Verify(r.Context(), b.Credential)
	if err != nil || email == "" {
		httpx.RespondError(w, r, httpx.ErrUnauthorized("Não foi possível validar seu login."))
		return
	}
	// Rate limit por email (o por IP já foi aplicado no middleware).
	if !h.limiter.AllowEmail(email) {
		httpx.RespondError(w, r, httpx.ErrTooManyRequests("Muitas tentativas. Aguarde um instante."))
		return
	}
	p, err := h.patients.RegisterFromPortal(r.Context(), service.PortalRegisterInput{
		Name: b.Name, Phone: b.Phone, Email: email, CPF: b.CPF,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	h.sess.Issue(w, email)
	httpx.Respond(w, r, http.StatusOK, "Cadastro realizado.", portalPatientView(p))
}

func portalPatientView(p *domain.Patient) map[string]any {
	return map[string]any{
		"id":    p.ID,
		"name":  p.Name,
		"phone": p.Phone,
		"email": p.Email,
		"cpf":   p.CPF,
	}
}

// me devolve o perfil do paciente autenticado (isolado pelo email da sessão).
func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	p, err := h.patients.GetByEmail(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Seu perfil.", portalPatientView(p))
}

type updateMeBody struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	CPF   string `json:"cpf"`
}

// updateMe atualiza os dados básicos do paciente autenticado.
func (h *Handlers) updateMe(w http.ResponseWriter, r *http.Request) {
	var b updateMeBody
	if err := httpx.DecodeJSON(r, &b); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	email := httpx.Actor(r.Context())
	p, err := h.patients.RegisterFromPortal(r.Context(), service.PortalRegisterInput{
		Name: b.Name, Phone: b.Phone, Email: email, CPF: b.CPF,
	})
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Perfil atualizado.", portalPatientView(p))
}

// mySessions devolve as sessões do paciente autenticado.
func (h *Handlers) mySessions(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	ss, err := h.sessions.MySessions(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Suas sessões.", ss)
}

// myDebts devolve os débitos do paciente autenticado.
func (h *Handlers) myDebts(w http.ResponseWriter, r *http.Request) {
	email := httpx.Actor(r.Context())
	ds, err := h.sessions.MyDebts(r.Context(), email)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httpx.Respond(w, r, http.StatusOK, "Seus débitos.", ds)
}
