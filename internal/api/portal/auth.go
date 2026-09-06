package portal

import (
	"context"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/service"
)

// IdentityVerifier valida uma credencial de login social (Google) e devolve o
// email verificado. Implementação real chama o userinfo do Google; nos testes,
// um fake devolve o email diretamente.
type IdentityVerifier interface {
	// Verify recebe o token/credencial do login e devolve o email verificado.
	Verify(ctx context.Context, credential string) (email string, err error)
}

// Authenticator protege as rotas autenticadas do portal validando a sessão
// própria (cookie assinado). NÃO confia no Pangolin (que aqui só termina TLS).
type Authenticator struct {
	sessions *SessionManager
}

// NewAuthenticator cria o middleware de autenticação do portal.
func NewAuthenticator(sessions *SessionManager) *Authenticator {
	return &Authenticator{sessions: sessions}
}

// Middleware exige uma sessão válida e injeta o email verificado no contexto.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, err := a.sessions.Verify(r)
		if err != nil {
			httpx.RespondError(w, r, httpx.ErrUnauthorized("Faça login para acessar sua área."))
			return
		}
		ctx := httpx.WithActor(r.Context(), email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ApprovalGate impõe, no servidor, que o paciente esteja aprovado antes de
// acessar as rotas de recurso do portal (defense in depth — R1.2). Encadeado
// SEMPRE depois do Authenticator (depende do email no contexto).
type ApprovalGate struct {
	patients *service.PatientService
}

// NewApprovalGate cria o middleware de gate de aprovação.
func NewApprovalGate(patients *service.PatientService) *ApprovalGate {
	return &ApprovalGate{patients: patients}
}

// Middleware nega (403) o acesso a paciente pendente/rejeitado e libera o
// aprovado. Resolve o paciente pelo email verificado da sessão.
func (a *ApprovalGate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := httpx.Actor(r.Context())
		p, err := a.patients.GetByEmail(r.Context(), email)
		if err != nil {
			// Sem cadastro concluído ainda: trata como não liberado.
			httpx.RespondError(w, r, httpx.ErrForbidden("Seu cadastro está em análise. Você poderá acessar assim que o terapeuta aprovar."))
			return
		}
		if !p.IsApproved() {
			httpx.RespondError(w, r, httpx.ErrForbidden("Seu cadastro está em análise. Você poderá acessar assim que o terapeuta aprovar."))
			return
		}
		next.ServeHTTP(w, r)
	})
}
