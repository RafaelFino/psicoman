package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	authEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint = "https://oauth2.googleapis.com/token"
)

// OAuthConfig são as credenciais e escopos do OAuth 3-legged.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// TokenStore persiste e recupera o refresh token (cifrado no repositório).
type TokenStore interface {
	// SaveRefreshToken guarda o refresh token e os escopos.
	SaveRefreshToken(ctx context.Context, refreshToken string, scopes []string) error
	// LoadRefreshToken devolve o refresh token; "" se não autorizado.
	LoadRefreshToken(ctx context.Context) (string, error)
	// SetReauthRequired sinaliza que a reautorização é necessária.
	SetReauthRequired(ctx context.Context, required bool) error
}

// OAuth gerencia o fluxo 3-legged e a renovação de access tokens.
type OAuth struct {
	cfg   OAuthConfig
	store TokenStore
	http  *http.Client

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

// NewOAuth cria o gerenciador OAuth.
func NewOAuth(cfg OAuthConfig, store TokenStore) *OAuth {
	return &OAuth{cfg: cfg, store: store, http: &http.Client{Timeout: 15 * time.Second}}
}

// AuthURL devolve a URL de autorização (consentimento do terapeuta).
func (o *OAuth) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", o.cfg.ClientID)
	q.Set("redirect_uri", o.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(o.cfg.Scopes, " "))
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("state", state)
	return authEndpoint + "?" + q.Encode()
}

// Exchange troca o authorization code por tokens e persiste o refresh token.
func (o *OAuth) Exchange(ctx context.Context, code string) error {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", o.cfg.ClientID)
	form.Set("client_secret", o.cfg.ClientSecret)
	form.Set("redirect_uri", o.cfg.RedirectURL)
	form.Set("grant_type", "authorization_code")

	tok, err := o.postToken(ctx, form)
	if err != nil {
		return err
	}
	if tok.RefreshToken == "" {
		return fmt.Errorf("google: refresh token ausente na resposta (verifique prompt=consent)")
	}
	o.setAccess(tok.AccessToken, tok.ExpiresIn)
	if err := o.store.SaveRefreshToken(ctx, tok.RefreshToken, o.cfg.Scopes); err != nil {
		return err
	}
	return o.store.SetReauthRequired(ctx, false)
}

// Token devolve um access token válido, renovando via refresh token se preciso.
// Implementa TokenSource.
func (o *OAuth) Token(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.accessToken != "" && time.Now().Before(o.expiry.Add(-30*time.Second)) {
		return o.accessToken, nil
	}
	refresh, err := o.store.LoadRefreshToken(ctx)
	if err != nil {
		return "", err
	}
	if refresh == "" {
		return "", fmt.Errorf("google: não autorizado (refresh token ausente)")
	}
	form := url.Values{}
	form.Set("client_id", o.cfg.ClientID)
	form.Set("client_secret", o.cfg.ClientSecret)
	form.Set("refresh_token", refresh)
	form.Set("grant_type", "refresh_token")

	tok, err := o.postToken(ctx, form)
	if err != nil {
		// Falha de refresh sinaliza reautorização, sem derrubar o processo.
		_ = o.store.SetReauthRequired(ctx, true)
		return "", err
	}
	o.setAccessLocked(tok.AccessToken, tok.ExpiresIn)
	return o.accessToken, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (o *OAuth) postToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google: troca de token falhou (%d)", resp.StatusCode)
	}
	return &tok, nil
}

func (o *OAuth) setAccess(token string, expiresIn int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.setAccessLocked(token, expiresIn)
}

func (o *OAuth) setAccessLocked(token string, expiresIn int) {
	o.accessToken = token
	o.expiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
}
