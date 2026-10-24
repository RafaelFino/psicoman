package portal

import (
	"context"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/api/httpx"
	"github.com/RafaelFino/psicoman/internal/api/service"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
)

// gateTimeout limita a resolução do paciente no gate.
const gateTimeout = 5 * time.Second

// RequireApproved devolve o middleware de gate de aprovação do grupo
// /v1/portal/* (Fase 5, reforçado na Fase 9). Deixa passar as rotas de cadastro/
// estado (register e me, acessíveis a pendente/rejeitado); para as demais
// resolve o paciente pela IDENTIDADE REAL da sessão do PocketBase
// (httpx.PortalAuthRecordID → app.PortalAuth.PatientIDByAuthID) e exige
// approval_status == "aprovado", senão responde 403 (acesso_negado).
//
// Vive no pacote portal (não em httpx) porque precisa importar service/app para
// resolver o paciente — a regra de dependência permite a internal/api importar
// app/service, mas httpx fica restrito a core+stdlib.
func RequireApproved(svc *service.Services) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if isPortalGateException(e.Request.Method, e.Request.URL.Path) {
			return e.Next() // register e me ficam fora do gate
		}
		// Identidade REAL da sessão do PocketBase (Fase 9): o id do auth record
		// patient_user. Sem sessão válida → 401.
		authID := httpx.PortalAuthRecordID(e)
		if authID == "" {
			return httpx.RespondError(e, http.StatusUnauthorized, "nao_autenticado",
				"Autenticação necessária.", nil)
		}
		ctx, cancel := context.WithTimeout(e.Request.Context(), gateTimeout)
		defer cancel()
		pid, err := svc.PortalAuth.PatientIDByAuthID(ctx, authID)
		if err != nil {
			return httpx.Fail(e, err) // login sem paciente → 404/erro
		}
		p, err := svc.Patients.Get(ctx, pid)
		if err != nil {
			return httpx.Fail(e, err) // ErrNotFound → 404
		}
		if p.ApprovalStatus != patient.PatientApproved {
			return httpx.RespondError(e, http.StatusForbidden, "acesso_negado",
				"Seu cadastro ainda não foi aprovado.", nil)
		}
		httpx.SetPortalPatientID(e, p.ID) // disponível ao handler
		return e.Next()
	}
}

// isPortalGateException indica se (método, path) é uma rota livre do gate. Os
// paths do portal são fixos e conhecidos; a comparação é por path EXATO para
// não deixar uma rota futura terminada em /portal/register ou /portal/me herdar
// a exceção por sufixo. O router monta as rotas sob /v1/portal sem barra final,
// então /v1/portal/me/ (barra final) NÃO é exceção e cai no gate.
func isPortalGateException(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/v1/portal/register":
		return true
	case method == http.MethodGet && path == "/v1/portal/me":
		return true
	default:
		return false
	}
}
