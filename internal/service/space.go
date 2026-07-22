package service

import (
	"errors"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type SpaceService struct{}

type CreateSpaceInput struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Type             string `json:"type"`
	CostCentsPerUse  int64  `json:"cost_cents_per_use"`
	CostCentsMonthly int64  `json:"cost_cents_monthly"`
	Notes            string `json:"notes"`
}

type CreateSpaceBookingInput struct {
	SpaceID       string `json:"space_id"`
	AppointmentID string `json:"appointment_id"`
	BookingDate   string `json:"booking_date"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

func (s *SpaceService) ListSpaces(db *storage.DB) ([]domain.TherapySpace, error) {
	return db.ListSpaces()
}

func (s *SpaceService) CreateSpace(db *storage.DB, in CreateSpaceInput) (*domain.TherapySpace, error) {
	if in.Name == "" {
		return nil, errors.New("nome é obrigatório")
	}
	spaceType := domain.SpaceType(in.Type)
	if spaceType == "" {
		spaceType = domain.SpaceFixed
	}
	return db.CreateSpace(domain.TherapySpace{
		Name:             in.Name,
		Address:          in.Address,
		Type:             spaceType,
		CostCentsPerUse:  in.CostCentsPerUse,
		CostCentsMonthly: in.CostCentsMonthly,
		IsAvailable:      true,
		Notes:            in.Notes,
	})
}

func (s *SpaceService) UpdateSpace(db *storage.DB, id string, in CreateSpaceInput) error {
	existing, err := db.GetSpace(id)
	if err != nil {
		return errors.New("espaço não encontrado")
	}
	existing.Name = in.Name
	existing.Address = in.Address
	if in.Type != "" {
		existing.Type = domain.SpaceType(in.Type)
	}
	existing.CostCentsPerUse = in.CostCentsPerUse
	existing.CostCentsMonthly = in.CostCentsMonthly
	existing.Notes = in.Notes
	return db.UpdateSpace(*existing)
}

func (s *SpaceService) DeleteSpace(db *storage.DB, id string) error {
	return db.DeleteSpace(id)
}

func (s *SpaceService) ListBookings(db *storage.DB, spaceID, date string) ([]domain.SpaceBooking, error) {
	return db.ListSpaceBookings(spaceID, date)
}

func (s *SpaceService) CreateBooking(db *storage.DB, in CreateSpaceBookingInput) (*domain.SpaceBooking, error) {
	if in.SpaceID == "" {
		return nil, errors.New("space_id é obrigatório")
	}
	if in.BookingDate == "" || in.StartTime == "" || in.EndTime == "" {
		return nil, errors.New("data, início e fim são obrigatórios")
	}
	if _, err := db.GetSpace(in.SpaceID); err != nil {
		return nil, errors.New("espaço não encontrado")
	}
	return db.CreateSpaceBooking(domain.SpaceBooking{
		SpaceID:       in.SpaceID,
		AppointmentID: in.AppointmentID,
		BookingDate:   in.BookingDate,
		StartTime:     in.StartTime,
		EndTime:       in.EndTime,
	})
}

func (s *SpaceService) DeleteBooking(db *storage.DB, id string) error {
	return db.DeleteSpaceBooking(id)
}
