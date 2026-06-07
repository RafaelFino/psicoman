package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCanCancel(t *testing.T) {
	rules := SchedulingRules{MinHoursToCancel: 24, AllowPatientCancel: true}
	now := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	appt := Appointment{Status: StatusScheduled, ScheduledAt: now.Add(48 * time.Hour)}

	assert.NoError(t, CanCancel(rules, appt, true, now))

	rules.AllowPatientCancel = false
	assert.ErrorIs(t, CanCancel(rules, appt, true, now), ErrCancelNotAllowed)

	rules.AllowPatientCancel = true
	appt.ScheduledAt = now.Add(12 * time.Hour)
	assert.ErrorIs(t, CanCancel(rules, appt, true, now), ErrMinHoursNotMet)
}

func TestCanReschedule(t *testing.T) {
	rules := SchedulingRules{MinHoursToReschedule: 24, MaxReschedulesPerMonth: 2, AllowPatientReschedule: true}
	now := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	appt := Appointment{Status: StatusScheduled, ScheduledAt: now.Add(72 * time.Hour)}

	assert.NoError(t, CanReschedule(rules, appt, 0, true, now))
	assert.ErrorIs(t, CanReschedule(rules, appt, 2, true, now), ErrMaxReschedulesReached)
}
