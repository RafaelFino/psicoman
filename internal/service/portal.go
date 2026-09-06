package service

import (
	"context"

	"github.com/RafaelFino/psicoman/internal/domain"
)

// PortalSessionService serve os dados self-service do paciente, sempre isolados
// pelo email verificado (nunca por id de query) — psicoman-seguranca-lgpd.md.
type PortalSessionService struct {
	patients PatientRepository
	sessions SessionRepository
	debts    DebtRepository
}

// NewPortalSessionService cria o serviço de leitura do portal.
func NewPortalSessionService(patients PatientRepository, sessions SessionRepository, debts DebtRepository) *PortalSessionService {
	return &PortalSessionService{patients: patients, sessions: sessions, debts: debts}
}

// resolvePatient devolve o paciente do email verificado.
func (s *PortalSessionService) resolvePatient(ctx context.Context, email string) (*domain.Patient, error) {
	return s.patients.GetByEmail(ctx, email)
}

// PortalSessionView é a visão de sessão exposta ao paciente (sem dado clínico).
type PortalSessionView struct {
	ID       string `json:"id"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Status   string `json:"status"`
	Modality string `json:"modality"`
	MeetURL  string `json:"meet_url"`
}

// MySessions devolve as sessões do paciente identificado pelo email.
func (s *PortalSessionService) MySessions(ctx context.Context, email string) ([]PortalSessionView, error) {
	p, err := s.resolvePatient(ctx, email)
	if err != nil {
		return nil, err
	}
	list, err := s.sessions.ListByPatient(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]PortalSessionView, 0, len(list))
	for _, ss := range list {
		out = append(out, PortalSessionView{
			ID:       ss.ID,
			StartsAt: ss.StartsAt.Format("2006-01-02T15:04:05-07:00"),
			EndsAt:   ss.EndsAt.Format("2006-01-02T15:04:05-07:00"),
			Status:   ss.Status,
			Modality: ss.Modality,
			MeetURL:  ss.MeetURL,
		})
	}
	return out, nil
}

// PortalDebtView é a visão de débito exposta ao paciente.
type PortalDebtView struct {
	ID      string `json:"id"`
	Amount  int64  `json:"amount"`
	Status  string `json:"status"`
	DueDate string `json:"due_date,omitempty"`
}

// MyDebts devolve os débitos do paciente identificado pelo email.
func (s *PortalSessionService) MyDebts(ctx context.Context, email string) ([]PortalDebtView, error) {
	p, err := s.resolvePatient(ctx, email)
	if err != nil {
		return nil, err
	}
	list, err := s.debts.ListByPatient(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]PortalDebtView, 0, len(list))
	for _, d := range list {
		v := PortalDebtView{ID: d.ID, Amount: d.Amount, Status: d.Status}
		if !d.DueDate.IsZero() {
			v.DueDate = d.DueDate.Format("2006-01-02")
		}
		out = append(out, v)
	}
	return out, nil
}
