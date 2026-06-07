package service

import (
	"context"
	"testing"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(t.TempDir(), "test")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndCancelAppointment(t *testing.T) {
	db := testDB(t)
	p, _ := db.CreatePatient(domain.Patient{Email: "c@test.com", Name: "Carol"})

	svc := &AppointmentService{Calendar: &NoopCalendar{}}
	at := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	appt, err := svc.Create(context.Background(), db, CreateAppointmentInput{
		PatientID: p.ID, Type: domain.AppointmentOnline, ScheduledAt: at,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, appt.MeetLink)

	err = svc.Cancel(context.Background(), db, appt.ID, "teste", false)
	require.NoError(t, err)

	updated, _ := db.GetAppointment(appt.ID)
	assert.Equal(t, domain.StatusCancelled, updated.Status)
}

func TestAvailableSlots(t *testing.T) {
	db := testDB(t)
	svc := &AppointmentService{}
	from := time.Now().UTC().Add(24 * time.Hour).Truncate(24 * time.Hour)
	to := from.Add(7 * 24 * time.Hour)

	slots, err := svc.AvailableSlots(db, from, to, 50)
	require.NoError(t, err)
	assert.NotEmpty(t, slots)
}
