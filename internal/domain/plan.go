package domain

import (
	"errors"
	"time"
)

// Tipos de plano (requirements §3.4).
const (
	PlanPorConsulta    = "pagamento_por_consulta"
	PlanPorMes         = "pagamento_por_mes"
	PlanFechadoMensal  = "plano_fechado_mensal"
	PlanFechadoTrim    = "plano_fechado_trimestral"
	PlanAtendimentoSoc = "atendimento_social"
)

// Plan é o acordo de pagamento de um paciente.
type Plan struct {
	ID        string
	PatientID string
	Type      string
	Amount    int64 // centavos (para planos com valor fixo)
	StartsAt  time.Time
	EndsAt    time.Time // zero = sem fim
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ValidPlanType indica se t é um tipo de plano conhecido.
func ValidPlanType(t string) bool {
	switch t {
	case PlanPorConsulta, PlanPorMes, PlanFechadoMensal, PlanFechadoTrim, PlanAtendimentoSoc:
		return true
	}
	return false
}

// IsPerSession indica se o débito é gerado por sessão finalizada.
func (p *Plan) IsPerSession() bool {
	return p.Type == PlanPorConsulta || p.Type == PlanPorMes
}

// IsFixedCycle indica se o débito é gerado por ciclo (mensal/trimestral).
func (p *Plan) IsFixedCycle() bool {
	return p.Type == PlanFechadoMensal || p.Type == PlanFechadoTrim
}

// IsSocial indica atendimento social (nunca gera débito).
func (p *Plan) IsSocial() bool {
	return p.Type == PlanAtendimentoSoc
}

// Validate verifica as invariantes do plano.
func (p *Plan) Validate() error {
	if p.PatientID == "" {
		return errors.New("O plano precisa de um paciente.")
	}
	if !ValidPlanType(p.Type) {
		return errors.New("Tipo de plano inválido.")
	}
	if p.IsFixedCycle() && p.Amount <= 0 {
		return errors.New("Planos fechados exigem um valor fixo maior que zero.")
	}
	if p.Amount < 0 {
		return errors.New("O valor não pode ser negativo.")
	}
	if p.StartsAt.IsZero() {
		return errors.New("O início da vigência é obrigatório.")
	}
	return nil
}

// ActiveOn indica se o plano está vigente na data ref.
func (p *Plan) ActiveOn(ref time.Time) bool {
	if ref.Before(p.StartsAt) {
		return false
	}
	if !p.EndsAt.IsZero() && ref.After(p.EndsAt) {
		return false
	}
	return true
}
