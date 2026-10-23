package billing

import (
	"errors"
	"time"
)

// Contract formaliza o acordo com o paciente (vigência, regra de reajuste,
// plano associado e referência ao artefato no GED). Entidade nova desta onda
// (D4b.2); a regra de cobrança permanece ancorada no Plan.
type Contract struct {
	ID             string
	PatientID      string
	PlanID         string    // plano associado (opcional)
	StartsAt       time.Time // início da vigência
	EndsAt         time.Time // zero = sem fim
	AdjustmentRule string    // regra de reajuste (texto livre, opcional)
	ArtifactFileID string    // ged_file do PDF/JSON do contrato (opcional)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate verifica as invariantes de shape do contrato.
func (c *Contract) Validate() error {
	if c.PatientID == "" {
		return errors.New("O contrato precisa de um paciente.")
	}
	if c.StartsAt.IsZero() {
		return errors.New("O início da vigência é obrigatório.")
	}
	if !c.EndsAt.IsZero() && !c.EndsAt.After(c.StartsAt) {
		return errors.New("O fim da vigência deve ser posterior ao início.")
	}
	return nil
}

// ActiveOn indica se o contrato está vigente na data ref.
func (c *Contract) ActiveOn(ref time.Time) bool {
	if ref.Before(c.StartsAt) {
		return false
	}
	if !c.EndsAt.IsZero() && ref.After(c.EndsAt) {
		return false
	}
	return true
}
