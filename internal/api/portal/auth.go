package portal

import (
	"context"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
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
