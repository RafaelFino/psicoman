package storage

import (
	"testing"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir(), "test")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPatientCRUD(t *testing.T) {
	db := testDB(t)
	p, err := db.CreatePatient(domain.Patient{Email: "a@test.com", Name: "Alice"})
	require.NoError(t, err)

	got, err := db.GetPatient(p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)

	list, err := db.ListPatients()
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestAppointmentFlow(t *testing.T) {
	db := testDB(t)
	p, _ := db.CreatePatient(domain.Patient{Email: "b@test.com", Name: "Bob"})

	at := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	appt, err := db.CreateAppointment(domain.Appointment{
		PatientID: p.ID, Type: domain.AppointmentOnline,
		Status: domain.StatusScheduled, ScheduledAt: at, DurationMinutes: 50,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, appt.ID)

	conflict, err := db.HasConflict(at, 50, "")
	require.NoError(t, err)
	assert.True(t, conflict)
}

func TestSchedulingRules(t *testing.T) {
	db := testDB(t)
	rules, err := db.GetSchedulingRules()
	require.NoError(t, err)
	assert.Equal(t, 24, rules.MinHoursToCancel)

	rules.MinHoursToCancel = 48
	require.NoError(t, db.UpdateSchedulingRules(rules))

	updated, err := db.GetSchedulingRules()
	require.NoError(t, err)
	assert.Equal(t, 48, updated.MinHoursToCancel)
}
