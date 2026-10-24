package portal

import (
	"context"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
)

const handlerTimeout = 5 * time.Second

// Handlers agrupa os serviços usados pelos handlers do ator portal.
type Handlers struct {
	svc *service.Services
}

// Mount registra as rotas /v1/portal/* no grupo informado.
func Mount(g *router.RouterGroup[*core.RequestEvent], svc *service.Services) {
	h := &Handlers{svc: svc}
	g.GET("/me", h.Me)
	g.GET("/sessions", h.Sessions)
	g.POST("/register", h.Register)
}

// Me GET /v1/portal/me — fora do gate de aprovação (o paciente vê o próprio
// estado). Identidade pelo seam httpx.PortalEmail (header; TODO Fase 9: sessão);
// 401 se ausente.
func (h *Handlers) Me(e *core.RequestEvent) error {
	email := httpx.PortalEmail(e)
	if email == "" {
		return httpx.RespondError(e, 401, "nao_autenticado", "Autenticação necessária.", nil)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.GetByEmail(ctx, email)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Seus dados.", toMeDTO(p))
}

// Sessions GET /v1/portal/sessions — agenda do paciente (sem conteúdo clínico).
func (h *Handlers) Sessions(e *core.RequestEvent) error {
	email := httpx.PortalEmail(e)
	if email == "" {
		return httpx.RespondError(e, 401, "nao_autenticado", "Autenticação necessária.", nil)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.GetByEmail(ctx, email)
	if err != nil {
		return httpx.Fail(e, err)
	}
	ss, err := h.svc.Sessions.ListByPatient(ctx, p.ID)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Suas sessões.", toSessionDTOs(ss))
}

type registerBody struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	CPF   string `json:"cpf"`
}

// Register POST /v1/portal/register — auto-cadastro do paciente (nasce pendente).
func (h *Handlers) Register(e *core.RequestEvent) error {
	var body registerBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.svc.Patients.RegisterFromPortal(ctx, app.PortalRegisterInput{
		Name:  body.Name,
		Phone: body.Phone,
		Email: body.Email,
		CPF:   body.CPF,
	})
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Cadastro recebido. Aguarde aprovação.", toMeDTO(p))
}

func reqCtx(e *core.RequestEvent) (context.Context, context.CancelFunc) {
	return context.WithTimeout(e.Request.Context(), handlerTimeout)
}
