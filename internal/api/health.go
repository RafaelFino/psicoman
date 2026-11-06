package api

import (
	"context"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/app"
)

// readyzTimeout limita os checks de readiness (cifra + DB + OAuth).
const readyzTimeout = 3 * time.Second

// livez GET /v1/livez — liveness: o processo está vivo e atendendo. 200 fixo,
// sem checar dependências (R5.1). Fora da auth (probe de orquestrador).
func livez(e *core.RequestEvent) error {
	return httpx.RespondOK(e, "Vivo.", map[string]any{"status": "alive"})
}

// readyz devolve o handler de readiness (/v1/readyz). Checa, em ordem: cifra
// montada (round-trip seal/open), DB acessível (lookup de coleção) e o estado
// do OAuth do Google (ok|degraded). Pronto quando cifra+DB ok; o Google
// degradado NÃO torna a instância "não-pronta" (a app opera sem Google, só
// sinaliza), mas é reportado no corpo. Falha de cifra ou DB → 503 (R5.1).
func readyz(svc *service.Services) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		ctx, cancel := context.WithTimeout(e.Request.Context(), readyzTimeout)
		defer cancel()

		checks := map[string]any{}
		ready := true

		// 1. Cifra montada: round-trip determinístico (não cifra dado real).
		if err := checkCipher(ctx, svc.Cipher); err != nil {
			checks["cipher"] = "erro"
			ready = false
		} else {
			checks["cipher"] = "ok"
		}

		// 2. DB acessível: um lookup de coleção conhecida exercita o SQLite.
		if _, err := e.App.FindCollectionByNameOrId("patient"); err != nil {
			checks["db"] = "erro"
			ready = false
		} else {
			checks["db"] = "ok"
		}

		// 3. Estado do OAuth do Google (não bloqueia a prontidão — a app opera
		// sem Google, só degrada o Calendar/Drive).
		state, reason := svc.GoogleAuth.Status(ctx)
		checks["google"] = string(state)
		if reason != "" {
			checks["google_reason"] = reason
		}

		data := map[string]any{"ready": ready, "checks": checks}
		if !ready {
			return httpx.RespondError(e, http.StatusServiceUnavailable, "nao_pronto",
				"Instância não está pronta.", nil)
		}
		return httpx.RespondOK(e, "Pronto.", data)
	}
}

// checkCipher faz um round-trip seal→open com um texto fixo, provando que a
// cifra está montada e operante. Nunca toca dado real.
func checkCipher(ctx context.Context, c domainCipher) error {
	const probe = "readyz-probe"
	env, err := c.Seal(ctx, probe)
	if err != nil {
		return err
	}
	plain, err := c.Open(ctx, env)
	if err != nil {
		return err
	}
	if plain != probe {
		return errCipherMismatch
	}
	return nil
}

// domainCipher é o subconjunto da cifra usado no readiness (Seal/Open). Evita
// importar o pacote domain só para o alias; satisfeito por domain.Cipher.
type domainCipher interface {
	Seal(ctx context.Context, plaintext string) (string, error)
	Open(ctx context.Context, envelope string) (string, error)
}

// errCipherMismatch sinaliza round-trip de cifra inconsistente no readiness.
var errCipherMismatch = app.NewValidation("round-trip de cifra inconsistente")
