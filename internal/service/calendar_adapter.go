package service

import (
	"context"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
)

type CalendarPort interface {
	CreateEvent(ctx context.Context, appt domain.Appointment, patientName string) (eventID, meetLink string, err error)
	UpdateEvent(ctx context.Context, eventID string, appt domain.Appointment, patientName string) (meetLink string, err error)
	DeleteEvent(ctx context.Context, eventID string) error
}

type DBCalendar struct {
	Google *GoogleCalendar
	Noop   *NoopCalendar
	DB     *storage.DB
}

func (c *DBCalendar) CreateEvent(ctx context.Context, appt domain.Appointment, patientName string) (string, string, error) {
	if c.Google != nil {
		return c.Google.CreateEvent(ctx, c.DB, appt, patientName)
	}
	return c.Noop.CreateEvent(ctx, appt, patientName)
}

func (c *DBCalendar) UpdateEvent(ctx context.Context, eventID string, appt domain.Appointment, patientName string) (string, error) {
	if c.Google != nil {
		return c.Google.UpdateEvent(ctx, c.DB, eventID, appt, patientName)
	}
	return c.Noop.UpdateEvent(ctx, eventID, appt, patientName)
}

func (c *DBCalendar) DeleteEvent(ctx context.Context, eventID string) error {
	if c.Google != nil {
		return c.Google.DeleteEvent(ctx, c.DB, eventID)
	}
	return c.Noop.DeleteEvent(ctx, eventID)
}
