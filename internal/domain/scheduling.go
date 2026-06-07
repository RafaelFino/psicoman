package domain

import (
	"errors"
	"time"
)

var (
	ErrCancelNotAllowed      = errors.New("cancelamento não permitido")
	ErrRescheduleNotAllowed  = errors.New("reagendamento não permitido")
	ErrMinHoursNotMet        = errors.New("prazo mínimo não atingido")
	ErrMaxReschedulesReached = errors.New("limite de reagendamentos atingido")
)

func CanCancel(rules SchedulingRules, appt Appointment, byPatient bool, now time.Time) error {
	if byPatient && !rules.AllowPatientCancel {
		return ErrCancelNotAllowed
	}
	if appt.Status == StatusCancelled {
		return errors.New("atendimento já cancelado")
	}
	hours := now.Sub(appt.ScheduledAt).Hours()
	if hours < 0 && -hours < float64(rules.MinHoursToCancel) {
		return ErrMinHoursNotMet
	}
	return nil
}

func CanReschedule(rules SchedulingRules, appt Appointment, reschedulesThisMonth int, byPatient bool, now time.Time) error {
	if byPatient && !rules.AllowPatientReschedule {
		return ErrRescheduleNotAllowed
	}
	if appt.Status == StatusCancelled {
		return errors.New("atendimento cancelado")
	}
	hours := now.Sub(appt.ScheduledAt).Hours()
	if hours < 0 && -hours < float64(rules.MinHoursToReschedule) {
		return ErrMinHoursNotMet
	}
	if byPatient && reschedulesThisMonth >= rules.MaxReschedulesPerMonth {
		return ErrMaxReschedulesReached
	}
	return nil
}
