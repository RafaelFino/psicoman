package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Modalidade de atendimento.
const (
	ModalityPresencial = "presencial"
	ModalityOnline     = "online"
)

// Periodicidade de custo.
const (
	PeriodPorSessao = "por_sessao"
	PeriodDiario    = "diario"
	PeriodMensal    = "mensal"
	PeriodAnual     = "anual"
)

// Location é um local de atendimento com custo e periodicidade.
type Location struct {
	ID         string
	Name       string
	Address    string
	Modality   string
	CostAmount int64  // centavos
	CostPeriod string // por_sessao|diario|mensal|anual
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ValidModality indica se a modalidade é válida.
func ValidModality(m string) bool {
	return m == ModalityPresencial || m == ModalityOnline
}

// ValidPeriod indica se a periodicidade de custo é válida.
func ValidPeriod(p string) bool {
	switch p {
	case PeriodPorSessao, PeriodDiario, PeriodMensal, PeriodAnual:
		return true
	}
	return false
}

// Validate verifica as invariantes do local.
func (l *Location) Validate() error {
	if strings.TrimSpace(l.Name) == "" {
		return errors.New("O nome do local é obrigatório.")
	}
	if !ValidModality(l.Modality) {
		return errors.New("A modalidade deve ser 'presencial' ou 'online'.")
	}
	if !ValidPeriod(l.CostPeriod) {
		return errors.New("A periodicidade do custo é inválida.")
	}
	if l.CostAmount < 0 {
		return errors.New("O custo não pode ser negativo.")
	}
	return nil
}

// Availability é uma janela de disponibilidade de agenda de um local.
type Availability struct {
	ID         string
	LocationID string
	Weekday    int    // 0=domingo .. 6=sábado
	StartTime  string // "HH:MM"
	EndTime    string // "HH:MM"
	Capacity   int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Validate verifica as invariantes da disponibilidade.
func (a *Availability) Validate() error {
	if a.Weekday < 0 || a.Weekday > 6 {
		return errors.New("O dia da semana deve estar entre 0 (domingo) e 6 (sábado).")
	}
	if !timeRe.MatchString(a.StartTime) || !timeRe.MatchString(a.EndTime) {
		return errors.New("Os horários devem estar no formato HH:MM.")
	}
	if a.StartTime >= a.EndTime {
		return errors.New("O horário inicial deve ser anterior ao final.")
	}
	if a.Capacity < 1 {
		return errors.New("A capacidade deve ser pelo menos 1.")
	}
	return nil
}
