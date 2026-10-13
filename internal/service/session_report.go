package service

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// SessionAggregate é o resultado do motor de agregação: contagem por categoria
// (domain.SessionCategory) e o faturado das realizadas no período. Reutilizado
// pelas métricas do paciente (R6.4–R6.5) e pelo relatório por local (R7).
type SessionAggregate struct {
	Counts       map[string]int `json:"counts"`              // categoria -> nº de sessões
	Total        int            `json:"total"`               // total de sessões no recorte
	FaturadoReal int64          `json:"faturado_realizadas"` // soma dos débitos das realizadas (centavos)
}

// aggregateSessions é o motor puro: dado um conjunto de sessões e um mapa de
// faturado por sessão (centavos), devolve a contagem por categoria e o total
// faturado das realizadas. Sessões sem categoria conhecida caem em
// CategoryDesconhecida. Não faz I/O; é a base testável de R6.4/R7.
func aggregateSessions(sessions []*domain.Session, faturadoPorSessao map[string]int64) SessionAggregate {
	agg := SessionAggregate{Counts: map[string]int{}}
	for _, s := range sessions {
		cat := domain.SessionCategory(s)
		agg.Counts[cat]++
		agg.Total++
		if cat == domain.CategoryRealizada {
			agg.FaturadoReal += faturadoPorSessao[s.ID]
		}
	}
	return agg
}

// PatientSessionMetrics agrega as sessões do paciente no intervalo [from, to)
// (ou histórico completo quando ambos zero), com contagens por categoria,
// taxas zero-safe e o faturado das realizadas. Agrupa por starts_at no fuso do
// projeto (R6.4–R6.5).
type PatientMetrics struct {
	SessionAggregate
	TaxaComparecimento float64 `json:"taxa_comparecimento"` // fração 0..1
	TaxaCancelamento   float64 `json:"taxa_cancelamento"`   // fração 0..1
	From               string  `json:"from,omitempty"`
	To                 string  `json:"to,omitempty"`
}

// SessionMetricsService calcula métricas de sessão por paciente e por local,
// compartilhando o motor de agregação.
type SessionMetricsService struct {
	sessions  SessionRepository
	debts     DebtRepository
	locations LocationRepository
}

// NewSessionMetricsService cria o serviço de métricas de sessão.
func NewSessionMetricsService(sessions SessionRepository, debts DebtRepository, locations LocationRepository) *SessionMetricsService {
	return &SessionMetricsService{sessions: sessions, debts: debts, locations: locations}
}

// inRange indica se t está em [from, to); limites zero são "sem limite".
func inRange(t, from, to time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && !t.Before(to) {
		return false
	}
	return true
}

// PatientSessionMetrics devolve as métricas do paciente. from/to zero = histórico
// completo. O recorte é por starts_at.
func (s *SessionMetricsService) PatientSessionMetrics(ctx context.Context, patientID string, from, to time.Time) (*PatientMetrics, error) {
	all, err := s.sessions.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	var sessions []*domain.Session
	for _, sess := range all {
		if inRange(sess.StartsAt, from, to) {
			sessions = append(sessions, sess)
		}
	}
	// Faturado por sessão: soma dos débitos vinculados à sessão (do paciente).
	debts, err := s.debts.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	faturado := map[string]int64{}
	for _, d := range debts {
		if d.SessionID != "" {
			faturado[d.SessionID] += d.Amount
		}
	}
	agg := aggregateSessions(sessions, faturado)
	m := &PatientMetrics{SessionAggregate: agg}
	m.TaxaComparecimento = attendanceRate(agg.Counts)
	m.TaxaCancelamento = cancelRate(agg.Counts, agg.Total)
	return m, nil
}

// LocationReport é o relatório agregado de sessões de um local no período.
type LocationReport struct {
	SessionAggregate
	LocationID   string `json:"location_id"`
	LocationName string `json:"location_name"`
	From         string `json:"from"`
	To           string `json:"to"`
}

// LocationSessionReport agrega as sessões de um local em [from, to) por starts_at.
func (s *SessionMetricsService) LocationSessionReport(ctx context.Context, locationID string, from, to time.Time) (*LocationReport, error) {
	loc, err := s.locations.Get(ctx, locationID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.sessions.ListByLocationRange(ctx, locationID, from, to)
	if err != nil {
		return nil, err
	}
	// Faturado das realizadas: soma dos débitos vinculados às sessões do recorte.
	ids := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.ID)
	}
	faturado := map[string]int64{}
	if len(ids) > 0 {
		debts, derr := s.debts.ListBySessionIDs(ctx, ids)
		if derr != nil {
			return nil, derr
		}
		for _, d := range debts {
			if d.SessionID != "" {
				faturado[d.SessionID] += d.Amount
			}
		}
	}
	agg := aggregateSessions(sessions, faturado)
	rep := &LocationReport{SessionAggregate: agg, LocationID: loc.ID, LocationName: loc.Name}
	return rep, nil
}

// attendanceRate = realizadas / (realizadas + faltas + canceladas_paciente); zero-safe.
func attendanceRate(counts map[string]int) float64 {
	num := counts[domain.CategoryRealizada]
	den := counts[domain.CategoryRealizada] + counts[domain.CategoryFalta] + counts[domain.CategoryCanceladaPaciente]
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// cancelRate = (canceladas_paciente + canceladas_terapeuta + canceladas_sem_autor) / total; zero-safe.
func cancelRate(counts map[string]int, total int) float64 {
	if total == 0 {
		return 0
	}
	canc := counts[domain.CategoryCanceladaPaciente] + counts[domain.CategoryCanceladaTerapeuta] + counts[domain.CategoryCanceladaSemAutor]
	return float64(canc) / float64(total)
}
