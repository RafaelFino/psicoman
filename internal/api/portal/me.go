package portal

import (
	"context"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
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
// estado). Identidade pela sessão REAL do PocketBase (Fase 9); 401 se ausente.
func (h *Handlers) Me(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.resolvePatient(e, ctx)
	if err != nil {
		return err
	}
	return httpx.RespondOK(e, "Seus dados.", toMeDTO(p))
}

// resolvePatient resolve o paciente da sessão REAL (Fase 9): id do auth record
// (patient_user) → patient_id → patient. Já escreve a resposta de erro (401 sem
// sessão; Fail para os demais) e devolve err não-nil nesse caso — o handler só
// propaga. Usado pelas rotas do portal fora do gate (me) e dentro dele.
func (h *Handlers) resolvePatient(e *core.RequestEvent, ctx context.Context) (*patient.Patient, error) {
	authID := httpx.PortalAuthRecordID(e)
	if authID == "" {
		return nil, httpx.RespondError(e, 401, "nao_autenticado", "Autenticação necessária.", nil)
	}
	pid, err := h.svc.PortalAuth.PatientIDByAuthID(ctx, authID)
	if err != nil {
		return nil, httpx.Fail(e, err)
	}
	p, err := h.svc.Patients.Get(ctx, pid)
	if err != nil {
		return nil, httpx.Fail(e, err)
	}
	return p, nil
}

// Sessions GET /v1/portal/sessions — agenda do paciente (sem conteúdo clínico).
func (h *Handlers) Sessions(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	p, err := h.resolvePatient(e, ctx)
	if err != nil {
		return err
	}
	ss, err := h.svc.Sessions.ListByPatient(ctx, p.ID)
	if err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Suas sessões.", toSessionDTOs(ss))
}

type registerBody struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	Password string `json:"password"`
}

// minPortalPassword é o tamanho mínimo da senha do portal (alinhado ao default
// da coleção de auth do PocketBase).
const minPortalPassword = 8

// Register POST /v1/portal/register — auto-cadastro do paciente (nasce pendente)
// + criação da credencial de login (coleção de auth patient_user, Fase 9). O
// paciente só acessa recursos após aprovação (gate), mas já pode logar e ver o
// próprio estado em /me.
func (h *Handlers) Register(e *core.RequestEvent) error {
	var body registerBody
	if err := e.BindBody(&body); err != nil {
		return httpx.FailBadRequest(e, err)
	}
	if len(body.Password) < minPortalPassword {
		return httpx.RespondError(e, 422, "senha_fraca",
			"A senha deve ter ao menos 8 caracteres.", nil)
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
	// Cria a credencial de login vinculada ao paciente. O email é o mesmo do
	// cadastro (único na coleção auth). Falha aqui (ex.: email já tem login)
	// vira erro de conflito — o paciente já existe, mas o login não foi criado.
	if err := h.svc.PortalAuth.CreateLogin(ctx, p.ID, p.Email, body.Password); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondCreated(e, "Cadastro recebido. Aguarde aprovação.", toMeDTO(p))
}

func reqCtx(e *core.RequestEvent) (context.Context, context.CancelFunc) {
	return context.WithTimeout(e.Request.Context(), handlerTimeout)
}
