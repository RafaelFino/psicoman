package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// SessionMetricsService calcula métricas de sessão por paciente e por local,
// compartilhando o motor de agregação do domínio clinical. O serviço só
// coordena o I/O (fontes de sessões/débitos/locais).
// Portado de c3ab585:internal/service/session_report.go.
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
func (s *SessionMetricsService) PatientSessionMetrics(ctx context.Context, patientID string, from, to time.Time) (*clinical.PatientMetrics, error) {
	all, err := s.sessions.ListByPatient(ctx, patientID)
	if err != nil {
		return nil, err
	}
	var sessions []*scheduling.Session
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
	agg := clinical.AggregateSessions(sessions, faturado)
	m := &clinical.PatientMetrics{SessionAggregate: agg}
	m.TaxaComparecimento = clinical.AttendanceRate(agg.Counts)
	m.TaxaCancelamento = clinical.CancelRate(agg.Counts, agg.Total)
	return m, nil
}

// LocationReport é o relatório agregado de sessões de um local no período.
type LocationReport struct {
	clinical.SessionAggregate
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
	agg := clinical.AggregateSessions(sessions, faturado)
	rep := &LocationReport{SessionAggregate: agg, LocationID: loc.ID, LocationName: loc.Name}
	return rep, nil
}
