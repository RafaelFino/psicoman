package admin

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/integration/google"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// callbackPath é a rota de callback do OAuth, usada para montar o redirect_uri
// (precisa bater EXATAMENTE com o registrado no Google Cloud Console).
const callbackPath = "/v1/admin/google/callback"

// oauthRedirectURL monta o redirect_uri absoluto a partir do request (esquema +
// host + path de callback). Atrás do Pangolin, o host externo chega em
// X-Forwarded-Host/Proto; cai para o Host/scheme do request quando ausentes.
func oauthRedirectURL(e *core.RequestEvent) string {
	scheme := "https"
	if p := e.Request.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	} else if e.Request.TLS == nil {
		scheme = "http"
	}
	host := e.Request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = e.Request.Host
	}
	return scheme + "://" + host + callbackPath
}

// GoogleAuthorize GET /v1/admin/google/authorize — monta a URL de consentimento
// (access_type=offline + prompt=consent, escopos calendar+drive.file) e
// redireciona o operador ao Google. 422 se as credenciais do app não estão
// configuradas (sem client_id/secret não há fluxo).
func (h *Handlers) GoogleAuthorize(e *core.RequestEvent) error {
	if !h.svc.GoogleOAuth.Configured() {
		return httpx.RespondError(e, http.StatusUnprocessableEntity, "google_nao_configurado",
			"Credenciais do Google (client_id/client_secret) não configuradas.", nil)
	}
	flow := google.NewAuthFlow(h.svc.GoogleOAuth.ClientID, h.svc.GoogleOAuth.ClientSecret, oauthRedirectURL(e))
	state := ulid.New() // CSRF: valor opaco de uso único (eco no callback)
	url := flow.AuthCodeURL(state)
	// Redireciona o navegador do operador ao consentimento do Google.
	e.Response.Header().Set("Location", url)
	e.Response.WriteHeader(http.StatusFound)
	return nil
}

// GoogleCallback GET /v1/admin/google/callback?code=&state= — recebe o retorno
// do consentimento, troca o code pelo refresh token e o persiste CIFRADO (via
// GoogleAuthService → adapter). Em erro do Google (parâmetro error), responde
// 422. Sucesso → envelope OK (sem expor o token).
func (h *Handlers) GoogleCallback(e *core.RequestEvent) error {
	q := e.Request.URL.Query()
	if gerr := q.Get("error"); gerr != "" {
		return httpx.RespondError(e, http.StatusUnprocessableEntity, "google_autorizacao_negada",
			"Autorização negada no Google.", nil)
	}
	code := q.Get("code")
	if code == "" {
		return httpx.RespondError(e, http.StatusBadRequest, "requisicao_invalida",
			"Callback do Google sem código de autorização.", nil)
	}
	if !h.svc.GoogleOAuth.Configured() {
		return httpx.RespondError(e, http.StatusUnprocessableEntity, "google_nao_configurado",
			"Credenciais do Google (client_id/client_secret) não configuradas.", nil)
	}

	flow := google.NewAuthFlow(h.svc.GoogleOAuth.ClientID, h.svc.GoogleOAuth.ClientSecret, oauthRedirectURL(e))
	ctx, cancel := reqCtx(e)
	defer cancel()
	tok, err := flow.Exchange(ctx, code)
	if err != nil {
		// Não vazar detalhe do erro do Google ao usuário (pode conter segredo).
		e.App.Logger().Warn("Google OAuth: troca de code falhou", "erro", err.Error())
		return httpx.RespondError(e, http.StatusBadGateway, "google_troca_falhou",
			"Falha ao trocar o código de autorização com o Google.", nil)
	}
	if err := h.svc.GoogleAuth.SaveToken(ctx, app.SaveTokenInput{
		RefreshToken: tok.RefreshToken,
		Scopes:       google.AuthorizeScopes,
		Expiry:       tok.Expiry,
	}); err != nil {
		return httpx.Fail(e, err)
	}
	return httpx.RespondOK(e, "Autorização do Google concluída.", map[string]any{"status": "ok"})
}

// GoogleStatus GET /v1/admin/google/status — estado da autorização do Google
// (ok|degraded) para a operação, sem expor o token.
func (h *Handlers) GoogleStatus(e *core.RequestEvent) error {
	ctx, cancel := reqCtx(e)
	defer cancel()
	state, reason := h.svc.GoogleAuth.Status(ctx)
	data := map[string]any{"state": string(state)}
	if reason != "" {
		data["reason"] = reason
	}
	return httpx.RespondOK(e, "Estado da autorização do Google.", data)
}
