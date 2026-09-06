// Package admin contém handlers e middleware exclusivos do binário
// psicoman-admin (auth do Pangolin, rotas administrativas). Não é importado
// pelo portal (psicoman-golang.md: código exclusivo de um binário não vai para
// pacotes compartilhados).
package admin

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/config"
	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/logger"
	"github.com/RafaelFino/psicoman/internal/service"
)

// Authenticator valida o acesso administrativo por conta própria (defense in
// depth sobre o Pangolin): header de email == email do config e header de
// secret == secret do config (docs/architecture.md §4.5).
type Authenticator struct {
	cfg   config.AdminAuthConfig
	audit *service.AuditService
	log   *logger.Logger
	dev   bool
}

// NewAuthenticator cria o middleware de autenticação admin.
func NewAuthenticator(cfg config.AdminAuthConfig, audit *service.AuditService, log *logger.Logger) *Authenticator {
	return &Authenticator{cfg: cfg, audit: audit, log: log}
}

// EnableDev desliga a autenticação (modo de desenvolvimento local). Nunca deve
// ser usado em produção.
func (a *Authenticator) EnableDev() *Authenticator {
	a.dev = true
	return a
}

// Middleware devolve o middleware que protege as rotas administrativas.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Modo dev: injeta o ator sem validar credenciais.
		if a.dev {
			actor := a.cfg.Email
			if actor == "" {
				actor = "dev@local"
			}
			ctx := httpx.WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		email := r.Header.Get(a.cfg.EmailHeader)
		secret := r.Header.Get(a.cfg.SecretHeader)

		if !a.valid(email, secret) {
			a.recordFailure(r.Context(), email, r.URL.Path)
			httpx.RespondError(w, r, httpx.ErrUnauthorized(
				"Acesso não autorizado. Verifique suas credenciais de acesso."))
			return
		}

		ctx := httpx.WithActor(r.Context(), email)
		if err := a.audit.Record(ctx, email, domain.AuditActionLoginSuccess, "auth", "", map[string]any{
			"path": r.URL.Path,
		}); err != nil {
			a.log.Warn("falha ao registrar auditoria de login", "error", err)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// valid compara credenciais em tempo constante para evitar timing attacks.
func (a *Authenticator) valid(email, secret string) bool {
	if email == "" || secret == "" {
		return false
	}
	emailOK := subtle.ConstantTimeCompare([]byte(email), []byte(a.cfg.Email)) == 1
	secretOK := subtle.ConstantTimeCompare([]byte(secret), []byte(a.cfg.Secret)) == 1
	return emailOK && secretOK
}

func (a *Authenticator) recordFailure(ctx context.Context, email, path string) {
	// Nunca logar o secret; só o email tentado e o caminho.
	actor := email
	if actor == "" {
		actor = "desconhecido"
	}
	if err := a.audit.Record(ctx, actor, domain.AuditActionLoginFailure, "auth", "", map[string]any{
		"path": path,
	}); err != nil {
		a.log.Warn("falha ao registrar auditoria de login negado", "error", err)
	}
}
