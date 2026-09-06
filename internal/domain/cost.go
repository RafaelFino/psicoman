package domain

import (
	"errors"
	"strings"
	"time"
)

// Categorias de custo (requirements §3.5).
const (
	CostKindLocal      = "local"
	CostKindCRP        = "crp"
	CostKindInfra      = "infra"
	CostKindPlataforma = "plataforma"
)

// ValidCostKind indica se k é uma categoria de custo conhecida.
func ValidCostKind(k string) bool {
	switch k {
	case CostKindLocal, CostKindCRP, CostKindInfra, CostKindPlataforma:
		return true
	}
	return false
}

// CostCategory classifica itens de custo.
type CostCategory struct {
	ID        string
	Kind      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate verifica as invariantes da categoria.
func (c *CostCategory) Validate() error {
	if !ValidCostKind(c.Kind) {
		return errors.New("Categoria de custo inválida.")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("O nome da categoria é obrigatório.")
	}
	return nil
}

// CostItem é um item de custo com valor e periodicidade.
type CostItem struct {
	ID         string
	CategoryID string
	Name       string
	Amount     int64  // centavos
	Period     string // por_sessao|diario|mensal|anual
	OriginID   string // para custos de plataforma (ROI)
	LocationID string // para custos de local
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate verifica as invariantes do item de custo.
func (c *CostItem) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("O nome do item de custo é obrigatório.")
	}
	if !ValidPeriod(c.Period) {
		return errors.New("A periodicidade do custo é inválida.")
	}
	if c.Amount < 0 {
		return errors.New("O valor do custo não pode ser negativo.")
	}
	return nil
}

// SessionCost é o custo atribuído a uma sessão (direto ou por rateio), com
// snapshot auditável da base do rateio.
type SessionCost struct {
	ID           string
	SessionID    string
	Amount       int64  // centavos
	Method       string // direto|rateio
	BaseSnapshot string // JSON com a base do rateio
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Métodos de atribuição de custo.
const (
	CostMethodDireto = "direto"
	CostMethodRateio = "rateio"
)
