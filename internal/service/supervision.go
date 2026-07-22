package service

import (
	"errors"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type SupervisionService struct{}

type CreateSupervisorInput struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Specialty string `json:"specialty"`
	CRP       string `json:"crp"`
	Notes     string `json:"notes"`
}

type CreateSupervisionSessionInput struct {
	SupervisorID    string `json:"supervisor_id"`
	ScheduledAt     string `json:"scheduled_at"`
	DurationMinutes int    `json:"duration_minutes"`
	Topics          string `json:"topics"`
	CostCents       int64  `json:"cost_cents"`
}

type UpdateSupervisionSessionInput struct {
	NotesHTML       string `json:"notes_html"`
	Topics          string `json:"topics"`
	DurationMinutes int    `json:"duration_minutes"`
	CostCents       int64  `json:"cost_cents"`
	Status          string `json:"status"`
}

func (s *SupervisionService) ListSupervisors(db *storage.DB) ([]domain.Supervisor, error) {
	return db.ListSupervisors()
}

func (s *SupervisionService) CreateSupervisor(db *storage.DB, in CreateSupervisorInput) (*domain.Supervisor, error) {
	if in.Name == "" {
		return nil, errors.New("nome é obrigatório")
	}
	return db.CreateSupervisor(domain.Supervisor{
		Name:      in.Name,
		Email:     in.Email,
		Specialty: in.Specialty,
		CRP:       in.CRP,
		Notes:     in.Notes,
	})
}

func (s *SupervisionService) UpdateSupervisor(db *storage.DB, id string, in CreateSupervisorInput) error {
	existing, err := db.GetSupervisor(id)
	if err != nil {
		return errors.New("supervisor não encontrado")
	}
	existing.Name = in.Name
	existing.Email = in.Email
	existing.Specialty = in.Specialty
	existing.CRP = in.CRP
	existing.Notes = in.Notes
	return db.UpdateSupervisor(*existing)
}

func (s *SupervisionService) DeleteSupervisor(db *storage.DB, id string) error {
	return db.DeleteSupervisor(id)
}

func (s *SupervisionService) ListSessions(db *storage.DB, supervisorID string) ([]domain.SupervisionSession, error) {
	return db.ListSupervisionSessions(supervisorID)
}

func (s *SupervisionService) CreateSession(db *storage.DB, in CreateSupervisionSessionInput) (*domain.SupervisionSession, error) {
	if in.SupervisorID == "" {
		return nil, errors.New("supervisor_id é obrigatório")
	}
	if in.ScheduledAt == "" {
		return nil, errors.New("scheduled_at é obrigatório")
	}
	if _, err := db.GetSupervisor(in.SupervisorID); err != nil {
		return nil, errors.New("supervisor não encontrado")
	}

	scheduledAt := parseTimeString(in.ScheduledAt)
	return db.CreateSupervisionSession(domain.SupervisionSession{
		SupervisorID:    in.SupervisorID,
		ScheduledAt:     scheduledAt,
		DurationMinutes: in.DurationMinutes,
		Topics:          in.Topics,
		CostCents:       in.CostCents,
	})
}

func (s *SupervisionService) UpdateSession(db *storage.DB, id string, in UpdateSupervisionSessionInput) (*domain.SupervisionSession, error) {
	ss, err := db.GetSupervisionSession(id)
	if err != nil {
		return nil, errors.New("sessão não encontrada")
	}
	if in.NotesHTML != "" {
		ss.NotesHTML = in.NotesHTML
	}
	if in.Topics != "" {
		ss.Topics = in.Topics
	}
	if in.DurationMinutes > 0 {
		ss.DurationMinutes = in.DurationMinutes
	}
	if in.CostCents > 0 {
		ss.CostCents = in.CostCents
	}
	if in.Status != "" {
		ss.Status = domain.SupervisionStatus(in.Status)
	}
	if err := db.UpdateSupervisionSession(*ss); err != nil {
		return nil, err
	}
	return db.GetSupervisionSession(id)
}

func (s *SupervisionService) MonthlyHours(db *storage.DB, month, year int) (int, error) {
	return db.SupervisionHoursForMonth(month, year)
}
