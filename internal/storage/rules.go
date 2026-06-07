package storage

import "github.com/fino/psicoman/internal/domain"

func (db *DB) GetSchedulingRules() (domain.SchedulingRules, error) {
	var r domain.SchedulingRules
	var allowCancel, allowReschedule int
	err := db.QueryRow(
		`SELECT min_hours_to_cancel, min_hours_to_reschedule, max_reschedules_per_month, allow_patient_cancel, allow_patient_reschedule FROM scheduling_rules WHERE id=1`,
	).Scan(&r.MinHoursToCancel, &r.MinHoursToReschedule, &r.MaxReschedulesPerMonth, &allowCancel, &allowReschedule)
	if err != nil {
		return r, err
	}
	r.AllowPatientCancel = allowCancel == 1
	r.AllowPatientReschedule = allowReschedule == 1
	return r, nil
}

func (db *DB) UpdateSchedulingRules(r domain.SchedulingRules) error {
	_, err := db.Exec(
		`UPDATE scheduling_rules SET min_hours_to_cancel=?, min_hours_to_reschedule=?, max_reschedules_per_month=?, allow_patient_cancel=?, allow_patient_reschedule=? WHERE id=1`,
		r.MinHoursToCancel, r.MinHoursToReschedule, r.MaxReschedulesPerMonth, boolInt(r.AllowPatientCancel), boolInt(r.AllowPatientReschedule),
	)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
