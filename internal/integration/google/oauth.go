package google

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// OAuthCredentials são as credenciais do app + o refresh token do terapeuta
// (OAuth 3-legged, conta pessoal, sem domain-wide delegation — psicoman-google-api.md).
// O refresh token é persistido cifrado e carregado no boot; o access token é
// renovado em runtime pelo oauth2.TokenSource.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Valid reporta se há credenciais suficientes para montar um cliente autenticado.
func (c OAuthCredentials) Valid() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RefreshToken != ""
}

// NewOAuthHTTPClient monta um *http.Client autenticado com o escopo mínimo de
// Calendar (CalendarScope). Conveniência para o wiring do Calendar.
func NewOAuthHTTPClient(ctx context.Context, creds OAuthCredentials) *http.Client {
	return NewOAuthHTTPClientWithScopes(ctx, creds, CalendarScope)
}

// NewOAuthHTTPClientWithScopes monta um *http.Client que injeta o Authorization
// do Google e renova o access token automaticamente a partir do refresh token,
// com os escopos informados (sempre os MÍNIMOS necessários — nunca mais amplos
// que o preciso, psicoman-google-api.md). O ctx governa a vida das renovações
// (cada chamada de negócio deriva seu próprio timeout — psicoman-golang.md).
func NewOAuthHTTPClientWithScopes(ctx context.Context, creds OAuthCredentials, scopes ...string) *http.Client {
	cfg := &oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     googleoauth.Endpoint,
		Scopes:       scopes,
	}
	// Só o refresh token: o oauth2 troca por um access token na 1ª chamada.
	return cfg.Client(ctx, &oauth2.Token{RefreshToken: creds.RefreshToken})
}
