package service

import (
	"errors"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type SessionNoteService struct{}

type CreateSessionNoteInput struct {
	AppointmentID       string `json:"appointment_id"`
	ContentHTML         string `json:"content_html"`
	PrivateNotes        string `json:"private_notes"`
	DurationPatientMin  int    `json:"duration_patient_min"`
	DurationAnalysisMin int    `json:"duration_analysis_min"`
	DurationAdminMin    int    `json:"duration_admin_min"`
}

type UpdateSessionNoteInput struct {
	ContentHTML         string `json:"content_html"`
	PrivateNotes        string `json:"private_notes"`
	DurationPatientMin  int    `json:"duration_patient_min"`
	DurationAnalysisMin int    `json:"duration_analysis_min"`
	DurationAdminMin    int    `json:"duration_admin_min"`
}

func (s *SessionNoteService) List(db *storage.DB, patientID string) ([]domain.SessionNote, error) {
	return db.ListSessionNotes(patientID)
}

func (s *SessionNoteService) Get(db *storage.DB, id string) (*domain.SessionNote, error) {
	return db.GetSessionNote(id)
}

func (s *SessionNoteService) GetByAppointment(db *storage.DB, appointmentID string) (*domain.SessionNote, error) {
	return db.GetSessionNoteByAppointment(appointmentID)
}

func (s *SessionNoteService) Create(db *storage.DB, in CreateSessionNoteInput) (*domain.SessionNote, error) {
	if in.AppointmentID == "" {
		return nil, errors.New("appointment_id é obrigatório")
	}

	// Verify appointment exists and get patient_id
	appt, err := db.GetAppointment(in.AppointmentID)
	if err != nil {
		return nil, errors.New("atendimento não encontrado")
	}

	// Check if session note already exists for this appointment
	existing, _ := db.GetSessionNoteByAppointment(in.AppointmentID)
	if existing != nil {
		return nil, errors.New("evolução já registrada para este atendimento")
	}

	sn := domain.SessionNote{
		AppointmentID:       in.AppointmentID,
		PatientID:           appt.PatientID,
		ContentHTML:         in.ContentHTML,
		PrivateNotes:        in.PrivateNotes,
		DurationPatientMin:  in.DurationPatientMin,
		DurationAnalysisMin: in.DurationAnalysisMin,
		DurationAdminMin:    in.DurationAdminMin,
	}
	return db.CreateSessionNote(sn)
}

func (s *SessionNoteService) Update(db *storage.DB, id string, in UpdateSessionNoteInput) (*domain.SessionNote, error) {
	sn, err := db.GetSessionNote(id)
	if err != nil {
		return nil, errors.New("evolução não encontrada")
	}

	sn.ContentHTML = in.ContentHTML
	sn.PrivateNotes = in.PrivateNotes
	sn.DurationPatientMin = in.DurationPatientMin
	sn.DurationAnalysisMin = in.DurationAnalysisMin
	sn.DurationAdminMin = in.DurationAdminMin

	if err := db.UpdateSessionNote(*sn); err != nil {
		return nil, err
	}
	return db.GetSessionNote(id)
}

func (s *SessionNoteService) MonthlyHours(db *storage.DB, month, year int) (map[string]int, error) {
	patientMin, analysisMin, adminMin, err := db.SessionNoteHoursForMonth(month, year)
	if err != nil {
		return nil, err
	}
	return map[string]int{
		"patient_minutes":  patientMin,
		"analysis_minutes": analysisMin,
		"admin_minutes":    adminMin,
		"total_minutes":    patientMin + analysisMin + adminMin,
	}, nil
}
