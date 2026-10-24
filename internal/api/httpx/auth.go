package httpx

import (
	"net/http"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Config do header de identidade injetado pelo Pangolin na borda (Fase 5, 5.1).
//
// Segue o mesmo padrão das consts de cifra em wiring.go (nome da env em const,
// leitura via os.Getenv + strings.TrimSpace). Diferença deliberada vs. cifra:
// aqui há fallback para o default quando a env está vazia — o nome de um
// cabeçalho não é segredo, então não abortamos o boot por ausência.
const (
	// EnvPangolinIdentityHeader é a env que sobrescreve o nome do header.
	EnvPangolinIdentityHeader = "PSICOMAN_PANGOLIN_IDENTITY_HEADER"
	// DefaultPangolinIdentityHeader é o nome padrão do header de identidade.
	DefaultPangolinIdentityHeader = "X-Pangolin-User"
)

// PangolinHeaderName resolve o nome do header de identidade: lê a env
// EnvPangolinIdentityHeader e cai no default se vazia. Chamado no boot (não a
// cada request); a chain recebe o nome já resolvido via closure.
func PangolinHeaderName() string {
	if v := strings.TrimSpace(os.Getenv(EnvPangolinIdentityHeader)); v != "" {
		return v
	}
	return DefaultPangolinIdentityHeader
}

// RequirePangolinIdentity devolve um middleware que exige o header de identidade
// injetado pelo Pangolin; responde 401 (nao_autenticado) se ausente/vazio.
// headerName já vem resolvido (env ou default) no boot. A identidade validada é
// gravada no store (storeActorIdentity) para auditoria.
//
// Usado por /v1/app/* e /v1/admin/*: ambos ficam atrás do Pangolin e o app só
// exige a PRESENÇA do header. A política fina (admin mais restrito) é do
// gateway na borda, não do app. A admin UI nativa /_/ e a REST /api/* NÃO
// recebem este middleware (auth própria de superuser do Dashboard).
func RequirePangolinIdentity(headerName string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		id := strings.TrimSpace(e.Request.Header.Get(headerName))
		if id == "" {
			return RespondError(e, http.StatusUnauthorized, "nao_autenticado",
				"Autenticação necessária.", nil)
		}
		e.Set(storeActorIdentity, id)
		return e.Next()
	}
}

// ActorIdentity devolve a identidade do ator resolvida pelo middleware do
// Pangolin ("" se ausente).
//
// PREPARAÇÃO DE AUDITORIA (Fase 5): o e.Set é barato e correto e fica desde já,
// mas não há consumidor nesta fase — a auditoria de aprovação já é feita no
// PatientService. A amarração do campo `actor` do audit_log a esta identidade
// fica para a fase que tratar auditoria por request.
func ActorIdentity(e *core.RequestEvent) string {
	if v, ok := e.Get(storeActorIdentity).(string); ok {
		return v
	}
	return ""
}

// PortalAuthRecordID resolve a identidade REAL do paciente no portal (Fase 9):
// o id do auth record (coleção patient_user) carregado pela sessão do PocketBase
// via o middleware global loadAuthToken (Authorization: Bearer <token>). Vazio
// quando não há sessão válida.
//
// Substitui o SEAM X-Portal-Email das Fases 4/5: o gate de aprovação
// (portal.RequireApproved) e os handlers do portal resolvem o paciente a partir
// deste id (via a porta app.PortalAuth.PatientIDByAuthID). Só este ponto mudou.
func PortalAuthRecordID(e *core.RequestEvent) string {
	if e.Auth == nil {
		return ""
	}
	// Garante que a sessão é da coleção do paciente (não terapeuta/superuser).
	if e.Auth.Collection() == nil || e.Auth.Collection().Name != "patient_user" {
		return ""
	}
	return e.Auth.Id
}

// SetPortalPatientID grava no store o id do paciente resolvido pelo gate de
// aprovação do portal, disponibilizando-o ao handler.
func SetPortalPatientID(e *core.RequestEvent, id string) {
	e.Set(storePortalPatientID, id)
}

// PortalPatientID lê o id do paciente gravado pelo gate de aprovação ("" se
// ausente).
func PortalPatientID(e *core.RequestEvent) string {
	if v, ok := e.Get(storePortalPatientID).(string); ok {
		return v
	}
	return ""
}
