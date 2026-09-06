// Package domain contém as entidades e tipos do núcleo (domínio magro):
// carregam dados e invariantes básicas de shape, não orquestração de negócio
// (psicoman-golang.md).
package domain

import "time"

// AuditLog registra uma operação sensível (prontuário, débito, config,
// backup/restore, autenticação). Nunca guarda conteúdo clínico.
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
)
