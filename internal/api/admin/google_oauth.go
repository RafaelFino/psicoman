package admin

import (
	"crypto/subtle"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/integration/google"
)

// callbackPath é a rota de callback do OAuth, usada para montar o redirect_uri
// (precisa bater EXATAMENTE com o registrado no Google Cloud Console).
const callbackPath = "/v1/admin/google/callback"

// Cookie que amarra o fluxo OAuth ao NAVEGADOR que iniciou o authorize (R7.2).
const (
	oauthStateCookie     = "psicoman_oauth_state"
	oauthStateCookiePath = "/v1/admin/google"
	oauthStateMaxAge     = 600 // 10 min, igual ao TTL do store
)

// msgStateInvalido é a mensagem única para state ausente, divergente, expirado
// ou reutilizado (sem distinguir o motivo ao cliente).
const msgStateInvalido = "Estado da autorização inválido ou expirado. Inicie a autorização novamente."

// isSecureRequest indica HTTPS direto (TLS) ou atrás do proxy (X-Forwarded-Proto).
// O `Secure` do cookie é só endurecimento: a proteção real vem do uso único do
// state e do SameSite.
func isSecureRequest(e *core.RequestEvent) bool {
	return e.Request.TLS != nil || e.Request.Header.Get("X-Forwarded-Proto") == "https"
}

// setStateCookie grava (maxAge > 0) ou expira (maxAge < 0) o cookie do state.
func setStateCookie(e *core.RequestEvent, value string, maxAge int) {
	http.SetCookie(e.Response, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    value,
		Path:     oauthStateCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(e),
		SameSite: http.SameSiteLaxMode, // o callback é navegação top-level GET vinda do Google
	})
}

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
	// CSRF: state imprevisível, de uso único e com TTL (store em memória), e o
	// MESMO valor num cookie HttpOnly que amarra o fluxo ao navegador.
	state := h.svc.OAuthState.Issue()
	setStateCookie(e, state, oauthStateMaxAge)
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

	// O cookie é lido ANTES e expirado em TODOS os caminhos (uso único também
	// no navegador).
	var cookieState string
	if c, err := e.Request.Cookie(oauthStateCookie); err == nil {
		cookieState = c.Value
	}
	setStateCookie(e, "", -1)

	if gerr := q.Get("error"); gerr != "" {
		return httpx.RespondError(e, http.StatusUnprocessableEntity, "google_autorizacao_negada",
			"Autorização negada no Google.", nil)
	}

	// state: query == cookie (tempo constante) E emitido por nós, não expirado e
	// ainda não consumido. Qualquer falha => 400 com a mesma mensagem, sem
	// distinguir o motivo. Roda ANTES de code/Configured/troca: o Google real
	// nunca é tocado com um state inválido.
	queryState := q.Get("state")
	if queryState == "" || cookieState == "" ||
		subtle.ConstantTimeCompare([]byte(queryState), []byte(cookieState)) != 1 ||
		!h.svc.OAuthState.Consume(queryState) {
		return httpx.RespondError(e, http.StatusBadRequest, "state_invalido", msgStateInvalido, nil)
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
