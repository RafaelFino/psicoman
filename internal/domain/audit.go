package domain

import "time"

// AuditLog registra uma operação sensível (prontuário, débito, config,
// backup/restore, autenticação). Nunca guarda conteúdo clínico.
//
// É transversal aos bounded contexts: vive no pacote raiz domain (sem regra de
// negócio de context), consumido pelos serviços de app ao auditar operações.
type AuditLog struct {
	ID        string
	Actor     string // email do ator
	Action    string
	Entity    string
	EntityID  string
	Metadata  map[string]any
	CreatedAt time.Time
}

// Ações de auditoria padronizadas.
const (
	AuditActionLoginSuccess = "login_sucesso"
	AuditActionLoginFailure = "login_falha"
	AuditActionCreate       = "criar"
	AuditActionUpdate       = "atualizar"
	AuditActionDelete       = "remover"
	AuditActionRead         = "ler"
	AuditActionDebtGenerate = "debito_gerar"
	AuditActionDebtPay      = "debito_quitar"
	AuditActionBackup       = "backup"
	AuditActionRestore      = "restore"
	AuditActionConfig       = "config"
	AuditActionApprove      = "aprovar"
	AuditActionReject       = "rejeitar"
	// Ações de sessão (patient-timeline R6/R4.3). Sem conteúdo clínico em metadata.
	AuditActionSessionConfirm    = "session.confirm"
	AuditActionSessionCancel     = "session.cancel"
	AuditActionSessionReschedule = "session.reschedule"
)
