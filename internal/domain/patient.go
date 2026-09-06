package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Patient é o paciente. Obrigatórios: nome, telefone, email. CPF é opcional,
// mas exigido para emissão de recibo/Receita Saúde (requirements §3.1).
type Patient struct {
	ID             string
	Name           string
	Phone          string
	Email          string
	CPF            string // vazio = não informado
	OriginID       string // canal de aquisição (opcional)
	ApprovalStatus string // pendente | aprovado | rejeitado (gate de acesso do portal)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Estados de aprovação do paciente (gate de acesso do portal — mvp-audit1 R1).
const (
	PatientPending  = "pendente"
	PatientApproved = "aprovado"
	PatientRejected = "rejeitado"
)

// ValidApprovalStatus indica se o estado de aprovação é conhecido.
func ValidApprovalStatus(s string) bool {
	switch s {
	case PatientPending, PatientApproved, PatientRejected:
		return true
	default:
		return false
	}
}

// IsApproved indica se o paciente já teve o acesso ao portal liberado.
func (p *Patient) IsApproved() bool { return p.ApprovalStatus == PatientApproved }

// CanTransitionApproval indica se é possível transitar do estado atual para
// next. Só `pendente` transita (→ aprovado|rejeitado); estados finais não
// retrocedem por padrão (reavaliação futura fora de escopo — design D1.1).
func (p *Patient) CanTransitionApproval(next string) bool {
	if !ValidApprovalStatus(next) {
		return false
	}
	if p.ApprovalStatus == PatientPending {
		return next == PatientApproved || next == PatientRejected
	}
	return false
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ErrCPFObrigatorio indica operação que exige CPF (recibo/Receita Saúde) sem CPF.
var ErrCPFObrigatorio = errors.New("cpf obrigatório para esta operação")

// Validate verifica as invariantes de shape do paciente (domínio magro).
// Devolve mensagem PT-BR no erro para uso direto na API.
func (p *Patient) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("O nome é obrigatório.")
	}
	if strings.TrimSpace(p.Phone) == "" {
		return errors.New("O telefone é obrigatório.")
	}
	if strings.TrimSpace(p.Email) == "" {
		return errors.New("O email é obrigatório.")
	}
	if !emailRe.MatchString(p.Email) {
		return errors.New("O email informado não é válido.")
	}
	if p.CPF != "" && !validCPF(p.CPF) {
		return errors.New("O CPF informado não é válido.")
	}
	if p.ApprovalStatus != "" && !ValidApprovalStatus(p.ApprovalStatus) {
		return errors.New("O estado de aprovação é inválido.")
	}
	return nil
}

// CanIssueReceipt indica se o paciente pode receber recibo/Receita Saúde.
func (p *Patient) CanIssueReceipt() bool { return p.CPF != "" }

// NormalizeCPF remove formatação do CPF (mantém só dígitos).
func NormalizeCPF(cpf string) string {
	var b strings.Builder
	for _, r := range cpf {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validCPF valida um CPF brasileiro pelos dígitos verificadores.
func validCPF(cpf string) bool {
	cpf = NormalizeCPF(cpf)
	if len(cpf) != 11 {
		return false
	}
	// Rejeita sequências repetidas (00000000000, 11111111111, ...).
	allSame := true
	for i := 1; i < 11; i++ {
		if cpf[i] != cpf[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	// Dígitos verificadores.
	for _, dv := range []int{9, 10} {
		sum := 0
		for i := 0; i < dv; i++ {
			sum += int(cpf[i]-'0') * (dv + 1 - i)
		}
		check := (sum * 10) % 11
		if check == 10 {
			check = 0
		}
		if check != int(cpf[dv]-'0') {
			return false
		}
	}
	return true
}

// Origin é um canal de aquisição de paciente (Doctoralia, indicação, etc.).
type Origin struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes da origem.
func (o *Origin) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return errors.New("O nome da origem é obrigatório.")
	}
	return nil
}
