package service

import (
	"errors"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type PatientService struct{}

type RegisterPatientInput struct {
	Email     string
	Name      string
	Phone     string
	Anamnesis string
	GoogleSub string
}

func (s *PatientService) List(db *storage.DB) ([]domain.Patient, error) {
	return db.ListPatients()
}

func (s *PatientService) Get(db *storage.DB, id string) (*domain.Patient, error) {
	return db.GetPatient(id)
}

func (s *PatientService) Register(db *storage.DB, in RegisterPatientInput) (*domain.Patient, error) {
	if in.Email == "" || in.Name == "" {
		return nil, errors.New("email e nome são obrigatórios")
	}
	if existing, _ := db.GetPatientByEmail(in.Email); existing != nil {
		return nil, errors.New("email já cadastrado")
	}
	return db.CreatePatient(domain.Patient{
		Email:     in.Email,
		Name:      in.Name,
		Phone:     in.Phone,
		Anamnesis: in.Anamnesis,
		GoogleSub: in.GoogleSub,
	})
}

func (s *PatientService) UpdateAnamnesis(db *storage.DB, patientID, anamnesis string) error {
	p, err := db.GetPatient(patientID)
	if err != nil {
		return err
	}
	p.Anamnesis = anamnesis
	return db.UpdatePatient(*p)
}

func (s *PatientService) FullReport(db *storage.DB, patientID string) (map[string]any, error) {
	p, err := db.GetPatient(patientID)
	if err != nil {
		return nil, err
	}
	from := p.CreatedAt
	to := from.AddDate(10, 0, 0)
	appts, _ := db.ListAppointments(from, to, patientID)
	docs, _ := db.ListDocuments(patientID)
	payments, _ := db.ListPayments(int(from.Month()), from.Year())

	return map[string]any{
		"patient":      p,
		"appointments": appts,
		"documents":    docs,
		"payments":     payments,
	}, nil
}
