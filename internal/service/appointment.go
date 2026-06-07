package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type AppointmentService struct {
	Calendar CalendarPort
}

type CreateAppointmentInput struct {
	PatientID       string
	Type            domain.AppointmentType
	ScheduledAt     time.Time
	DurationMinutes int
	Notes           string
}

func (s *AppointmentService) List(db *storage.DB, from, to time.Time, patientID string) ([]domain.Appointment, error) {
	return db.ListAppointments(from, to, patientID)
}

func (s *AppointmentService) Get(db *storage.DB, id string) (*domain.Appointment, error) {
	return db.GetAppointment(id)
}

func (s *AppointmentService) Create(ctx context.Context, db *storage.DB, in CreateAppointmentInput) (*domain.Appointment, error) {
	if in.DurationMinutes == 0 {
		in.DurationMinutes = 50
	}
	conflict, err := db.HasConflict(in.ScheduledAt, in.DurationMinutes, "")
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, errors.New("horário indisponível")
	}

	patient, err := db.GetPatient(in.PatientID)
	if err != nil {
		return nil, fmt.Errorf("paciente não encontrado: %w", err)
	}

	appt := domain.Appointment{
		PatientID:       in.PatientID,
		Type:            in.Type,
		Status:          domain.StatusScheduled,
		ScheduledAt:     in.ScheduledAt,
		DurationMinutes: in.DurationMinutes,
		Notes:           in.Notes,
	}

	created, err := db.CreateAppointment(appt)
	if err != nil {
		return nil, err
	}

	if s.Calendar != nil {
		eventID, meetLink, err := s.Calendar.CreateEvent(ctx, *created, patient.Name)
		if err == nil {
			created.GoogleEventID = eventID
			created.MeetLink = meetLink
			_ = db.UpdateAppointment(*created)
		}
	}

	return db.GetAppointment(created.ID)
}

func (s *AppointmentService) Cancel(ctx context.Context, db *storage.DB, id, reason string, byPatient bool) error {
	appt, err := db.GetAppointment(id)
	if err != nil {
		return err
	}
	rules, err := db.GetSchedulingRules()
	if err != nil {
		return err
	}
	if err := domain.CanCancel(rules, *appt, byPatient, time.Now().UTC()); err != nil {
		return err
	}

	appt.Status = domain.StatusCancelled
	appt.CancellationReason = reason
	if err := db.UpdateAppointment(*appt); err != nil {
		return err
	}
	if s.Calendar != nil && appt.GoogleEventID != "" {
		_ = s.Calendar.DeleteEvent(ctx, appt.GoogleEventID)
	}
	return nil
}

func (s *AppointmentService) Reschedule(ctx context.Context, db *storage.DB, id string, newTime time.Time, byPatient bool) (*domain.Appointment, error) {
	appt, err := db.GetAppointment(id)
	if err != nil {
		return nil, err
	}
	rules, err := db.GetSchedulingRules()
	if err != nil {
		return nil, err
	}

	reschedules, _ := db.CountReschedulesThisMonth(appt.PatientID, int(newTime.Month()), newTime.Year())
	if err := domain.CanReschedule(rules, *appt, reschedules, byPatient, time.Now().UTC()); err != nil {
		return nil, err
	}

	conflict, err := db.HasConflict(newTime, appt.DurationMinutes, appt.ID)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, errors.New("horário indisponível")
	}

	appt.ScheduledAt = newTime
	appt.Status = domain.StatusRescheduled
	if err := db.UpdateAppointment(*appt); err != nil {
		return nil, err
	}

	if s.Calendar != nil && appt.GoogleEventID != "" {
		link, _ := s.Calendar.UpdateEvent(ctx, appt.GoogleEventID, *appt, appt.PatientName)
		appt.MeetLink = link
		_ = db.UpdateAppointment(*appt)
	}

	return db.GetAppointment(id)
}

func (s *AppointmentService) UpdateNotes(db *storage.DB, id, notes, reportHTML string) (*domain.Appointment, error) {
	appt, err := db.GetAppointment(id)
	if err != nil {
		return nil, err
	}
	appt.Notes = notes
	if reportHTML != "" {
		appt.ReportHTML = reportHTML
	}
	if err := db.UpdateAppointment(*appt); err != nil {
		return nil, err
	}
	return db.GetAppointment(id)
}

func (s *AppointmentService) Complete(db *storage.DB, id string) (*domain.Appointment, error) {
	appt, err := db.GetAppointment(id)
	if err != nil {
		return nil, err
	}
	appt.Status = domain.StatusCompleted
	if err := db.UpdateAppointment(*appt); err != nil {
		return nil, err
	}
	return db.GetAppointment(id)
}

func (s *AppointmentService) AvailableSlots(db *storage.DB, from, to time.Time, duration int) ([]domain.AvailableSlot, error) {
	if duration == 0 {
		duration = 50
	}
	appts, err := db.ListAppointments(from, to, "")
	if err != nil {
		return nil, err
	}

	workStart := 8
	workEnd := 18
	var slots []domain.AvailableSlot

	for d := from; d.Before(to); d = d.Add(24 * time.Hour) {
		if d.Weekday() == time.Sunday {
			continue
		}
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), workStart, 0, 0, 0, time.UTC)
		dayEnd := time.Date(d.Year(), d.Month(), d.Day(), workEnd, 0, 0, 0, time.UTC)

		for slot := dayStart; slot.Add(time.Duration(duration)*time.Minute).Before(dayEnd) || slot.Add(time.Duration(duration)*time.Minute).Equal(dayEnd); slot = slot.Add(time.Duration(duration) * time.Minute) {
			if slot.Before(time.Now().UTC()) {
				continue
			}
			busy := false
			slotEnd := slot.Add(time.Duration(duration) * time.Minute)
			for _, a := range appts {
				if a.Status == domain.StatusCancelled {
					continue
				}
				aEnd := a.ScheduledAt.Add(time.Duration(a.DurationMinutes) * time.Minute)
				if slot.Before(aEnd) && slotEnd.After(a.ScheduledAt) {
					busy = true
					break
				}
			}
			if !busy {
				slots = append(slots, domain.AvailableSlot{Start: slot, DurationMinutes: duration})
			}
		}
	}
	return slots, nil
}
