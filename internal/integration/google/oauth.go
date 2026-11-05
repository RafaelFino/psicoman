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

// AuthorizeScopes são os escopos pedidos no fluxo de autorização 3-legged da
// instância: Calendar (eventos/Meet) + drive.file (DR no Drive). São os MÍNIMOS
// necessários — nunca mais amplos (psicoman-google-api.md).
var AuthorizeScopes = []string{CalendarScope, DriveScope}

// authEndpoint é o endpoint OAuth usado pelo AuthFlow. Default: o endpoint real
// do Google. É um SEAM de pacote reatribuível pelos testes (em processo) para
// apontar a um servidor OAuth FAKE — nenhum teste toca o Google real
// (psicoman-google-api.md). Nunca reatribuído em produção.
var authEndpoint = googleoauth.Endpoint

// AuthFlow encapsula a configuração OAuth 3-legged para autorizar a instância:
// monta a URL de consentimento e troca o code pelo refresh token. Fica em
// integration/google (único lugar que conhece o SDK oauth2); o handler admin
// orquestra sem tocar a lib direto.
type AuthFlow struct {
	cfg *oauth2.Config
}

// NewAuthFlow cria o fluxo de autorização com o client_id/secret do app e o
// redirectURL (a rota de callback). Pede os AuthorizeScopes (calendar+drive.file).
func NewAuthFlow(clientID, clientSecret, redirectURL string) *AuthFlow {
	return &AuthFlow{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     authEndpoint,
		RedirectURL:  redirectURL,
		Scopes:       AuthorizeScopes,
	}}
}

// SetAuthEndpointForTest aponta o endpoint OAuth a um servidor fake (authURL +
// tokenURL). Devolve uma função de restauração. Uso EXCLUSIVO de teste — permite
// exercitar o fluxo de callback (troca code→token) sem a rede do Google.
func SetAuthEndpointForTest(authURL, tokenURL string) (restore func()) {
	prev := authEndpoint
	authEndpoint = oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL}
	return func() { authEndpoint = prev }
}

// AuthCodeURL monta a URL de consentimento. access_type=offline + prompt=consent
// garantem que o Google devolva um refresh token (sem o prompt, uma segunda
// autorização pode vir sem refresh token). state protege contra CSRF.
func (f *AuthFlow) AuthCodeURL(state string) string {
	return f.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"))
}

// Exchange troca o code recebido no callback por um token. O refresh token
// (campo de interesse) vem no token.RefreshToken; a expiry do access token em
// token.Expiry. ctx deve carregar timeout (chamada de rede ao Google).
func (f *AuthFlow) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return f.cfg.Exchange(ctx, code)
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
