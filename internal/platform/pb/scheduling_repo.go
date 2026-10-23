package pb

import (
	"context"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// sessionRepo implementa app.SessionRepository e app.CostSessionSource sobre a
// coleção session.
type sessionRepo struct{ app core.App }

var (
	_ app.SessionRepository = (*sessionRepo)(nil)
	_ app.CostSessionSource = (*sessionRepo)(nil)
)

// NewSessionRepository cria o repositório de sessões.
func NewSessionRepository(a core.App) *sessionRepo { return &sessionRepo{app: a} }

func (r *sessionRepo) toRecord(rec *core.Record, s *scheduling.Session) {
	rec.Set("id", s.ID)
	rec.Set("patient_id", s.PatientID)
	rec.Set("location_id", s.LocationID)
	rec.Set("request_id", s.RequestID)
	rec.Set("contract_id", s.ContractID)
	rec.Set("plan_id", s.PlanID)
	rec.Set("modality", s.Modality)
	rec.Set("starts_at", formatTime(s.StartsAt))
	rec.Set("ends_at", formatTime(s.EndsAt))
	rec.Set("status", s.Status)
	rec.Set("bill", s.Bill)
	rec.Set("consider_cost", s.ConsiderCost)
	rec.Set("google_event_id", s.GoogleEventID)
	rec.Set("meet_url", s.MeetURL)
	rec.Set("confirmed_at", formatTime(s.ConfirmedAt))
	rec.Set("canceled_by", s.CanceledBy)
	rec.Set("cancel_reason", s.CancelReason)
	rec.Set("canceled_at", formatTime(s.CanceledAt))
	rec.Set("created_at", formatTime(s.CreatedAt))
	rec.Set("updated_at", formatTime(s.UpdatedAt))
}

func sessionFromRecord(rec *core.Record) *scheduling.Session {
	return &scheduling.Session{
		ID:            rec.Id,
		PatientID:     rec.GetString("patient_id"),
		LocationID:    rec.GetString("location_id"),
		RequestID:     rec.GetString("request_id"),
		ContractID:    rec.GetString("contract_id"),
		PlanID:        rec.GetString("plan_id"),
		Modality:      rec.GetString("modality"),
		StartsAt:      parseTime(rec.GetString("starts_at")),
		EndsAt:        parseTime(rec.GetString("ends_at")),
		Status:        rec.GetString("status"),
		Bill:          rec.GetBool("bill"),
		ConsiderCost:  rec.GetBool("consider_cost"),
		GoogleEventID: rec.GetString("google_event_id"),
		MeetURL:       rec.GetString("meet_url"),
		ConfirmedAt:   parseTime(rec.GetString("confirmed_at")),
		CanceledBy:    rec.GetString("canceled_by"),
		CancelReason:  rec.GetString("cancel_reason"),
		CanceledAt:    parseTime(rec.GetString("canceled_at")),
		CreatedAt:     parseTime(rec.GetString("created_at")),
		UpdatedAt:     parseTime(rec.GetString("updated_at")),
	}
}

func (r *sessionRepo) Create(ctx context.Context, s *scheduling.Session) error {
	col, err := r.app.FindCollectionByNameOrId("session")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, s)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *sessionRepo) Update(ctx context.Context, s *scheduling.Session) error {
	rec, err := getRecordByID(r.app, "session", s.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, s)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *sessionRepo) Get(ctx context.Context, id string) (*scheduling.Session, error) {
	rec, err := r.app.FindFirstRecordByFilter("session", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return sessionFromRecord(rec), nil
}

func (r *sessionRepo) List(ctx context.Context) ([]*scheduling.Session, error) {
	return r.listByFilter(notDeletedFilter, nil)
}

func (r *sessionRepo) ListByPatient(ctx context.Context, patientID string) ([]*scheduling.Session, error) {
	return r.listByFilter("patient_id = {:p} && "+notDeletedFilter, dbxParams("p", patientID))
}

func (r *sessionRepo) ListByRange(ctx context.Context, from, to time.Time) ([]*scheduling.Session, error) {
	return r.listByFilter(
		"starts_at >= {:from} && starts_at < {:to} && "+notDeletedFilter,
		dbx.Params{"from": clock.Format(from), "to": clock.Format(to)})
}

func (r *sessionRepo) ListByLocationRange(ctx context.Context, locationID string, from, to time.Time) ([]*scheduling.Session, error) {
	filter := "location_id = {:loc} && " + notDeletedFilter
	params := dbx.Params{"loc": locationID}
	// Limites zero = sem limite (histórico completo do local).
	if !from.IsZero() {
		filter += " && starts_at >= {:from}"
		params["from"] = clock.Format(from)
	}
	if !to.IsZero() {
		filter += " && starts_at < {:to}"
		params["to"] = clock.Format(to)
	}
	return r.listByFilter(filter, params)
}

func (r *sessionRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "session", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

// CountRealizadasNoLocalNoPeriodo conta as sessões realizadas de um local em
// [de, ate), para o rateio proporcional de custo (app.CostSessionSource).
func (r *sessionRepo) CountRealizadasNoLocalNoPeriodo(ctx context.Context, locationID string, de, ate time.Time) (int, error) {
	recs, err := r.listByFilter(
		"location_id = {:loc} && status = {:status} && starts_at >= {:from} && starts_at < {:to} && "+notDeletedFilter,
		dbx.Params{
			"loc":    locationID,
			"status": scheduling.SessionRealizada,
			"from":   clock.Format(de),
			"to":     clock.Format(ate),
		})
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

func (r *sessionRepo) listByFilter(filter string, params dbx.Params) ([]*scheduling.Session, error) {
	var recs []*core.Record
	var err error
	if params == nil {
		recs, err = r.app.FindRecordsByFilter("session", filter, "starts_at", 0, 0)
	} else {
		recs, err = r.app.FindRecordsByFilter("session", filter, "starts_at", 0, 0, params)
	}
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*scheduling.Session, 0, len(recs))
	for _, rec := range recs {
		out = append(out, sessionFromRecord(rec))
	}
	return out, nil
}

// appointmentRepo implementa app.AppointmentRepository sobre appointment_request.
type appointmentRepo struct{ app core.App }

var _ app.AppointmentRepository = (*appointmentRepo)(nil)

// NewAppointmentRepository cria o repositório de pedidos de agendamento.
func NewAppointmentRepository(a core.App) app.AppointmentRepository { return &appointmentRepo{app: a} }

func (r *appointmentRepo) toRecord(rec *core.Record, a *scheduling.AppointmentRequest) {
	rec.Set("id", a.ID)
	rec.Set("patient_id", a.PatientID)
	rec.Set("location_id", a.LocationID)
	rec.Set("slot_start", formatTime(a.SlotStart))
	rec.Set("slot_end", formatTime(a.SlotEnd))
	rec.Set("status", a.Status)
	rec.Set("note", a.Note)
	rec.Set("created_at", formatTime(a.CreatedAt))
	rec.Set("updated_at", formatTime(a.UpdatedAt))
}

func appointmentFromRecord(rec *core.Record) *scheduling.AppointmentRequest {
	return &scheduling.AppointmentRequest{
		ID:         rec.Id,
		PatientID:  rec.GetString("patient_id"),
		LocationID: rec.GetString("location_id"),
		SlotStart:  parseTime(rec.GetString("slot_start")),
		SlotEnd:    parseTime(rec.GetString("slot_end")),
		Status:     rec.GetString("status"),
		Note:       rec.GetString("note"),
		CreatedAt:  parseTime(rec.GetString("created_at")),
		UpdatedAt:  parseTime(rec.GetString("updated_at")),
	}
}

func (r *appointmentRepo) Create(ctx context.Context, a *scheduling.AppointmentRequest) error {
	col, err := r.app.FindCollectionByNameOrId("appointment_request")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, a)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *appointmentRepo) Get(ctx context.Context, id string) (*scheduling.AppointmentRequest, error) {
	rec, err := getRecordByID(r.app, "appointment_request", id)
	if err != nil {
		return nil, err
	}
	return appointmentFromRecord(rec), nil
}

func (r *appointmentRepo) ListPending(ctx context.Context) ([]*scheduling.AppointmentRequest, error) {
	return r.listByFilter("status = {:s}", dbxParams("s", scheduling.ApptPendente))
}

func (r *appointmentRepo) ListByPatient(ctx context.Context, patientID string) ([]*scheduling.AppointmentRequest, error) {
	return r.listByFilter("patient_id = {:p}", dbxParams("p", patientID))
}

func (r *appointmentRepo) UpdateStatus(ctx context.Context, id, status string) error {
	rec, err := getRecordByID(r.app, "appointment_request", id)
	if err != nil {
		return err
	}
	rec.Set("status", status)
	rec.Set("updated_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *appointmentRepo) listByFilter(filter string, params dbx.Params) ([]*scheduling.AppointmentRequest, error) {
	recs, err := r.app.FindRecordsByFilter("appointment_request", filter, "slot_start", 0, 0, params)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*scheduling.AppointmentRequest, 0, len(recs))
	for _, rec := range recs {
		out = append(out, appointmentFromRecord(rec))
	}
	return out, nil
}

// locationRepo implementa app.LocationRepository sobre location + availability.
type locationRepo struct{ app core.App }

var _ app.LocationRepository = (*locationRepo)(nil)

// NewLocationRepository cria o repositório de locais e disponibilidades.
func NewLocationRepository(a core.App) app.LocationRepository { return &locationRepo{app: a} }

func (r *locationRepo) toRecord(rec *core.Record, l *scheduling.Location) {
	rec.Set("id", l.ID)
	rec.Set("name", l.Name)
	rec.Set("address", l.Address)
	rec.Set("modality", l.Modality)
	rec.Set("cost_amount", l.CostAmount)
	rec.Set("cost_period", l.CostPeriod)
	rec.Set("created_at", formatTime(l.CreatedAt))
	rec.Set("updated_at", formatTime(l.UpdatedAt))
}

func locationFromRecord(rec *core.Record) *scheduling.Location {
	return &scheduling.Location{
		ID:         rec.Id,
		Name:       rec.GetString("name"),
		Address:    rec.GetString("address"),
		Modality:   rec.GetString("modality"),
		CostAmount: rec.GetInt64("cost_amount"),
		CostPeriod: rec.GetString("cost_period"),
		CreatedAt:  parseTime(rec.GetString("created_at")),
		UpdatedAt:  parseTime(rec.GetString("updated_at")),
	}
}

func (r *locationRepo) Create(ctx context.Context, l *scheduling.Location) error {
	col, err := r.app.FindCollectionByNameOrId("location")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toRecord(rec, l)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *locationRepo) Update(ctx context.Context, l *scheduling.Location) error {
	rec, err := getRecordByID(r.app, "location", l.ID)
	if err != nil {
		return err
	}
	r.toRecord(rec, l)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *locationRepo) Get(ctx context.Context, id string) (*scheduling.Location, error) {
	rec, err := r.app.FindFirstRecordByFilter("location", "id = {:id} && "+notDeletedFilter, dbxParams("id", id))
	if err != nil {
		return nil, mapError(err)
	}
	return locationFromRecord(rec), nil
}

func (r *locationRepo) List(ctx context.Context) ([]*scheduling.Location, error) {
	recs, err := r.app.FindRecordsByFilter("location", notDeletedFilter, "name", 0, 0)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*scheduling.Location, 0, len(recs))
	for _, rec := range recs {
		out = append(out, locationFromRecord(rec))
	}
	return out, nil
}

func (r *locationRepo) SoftDelete(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "location", id)
	if err != nil {
		return err
	}
	rec.Set("deleted_at", clock.Format(clock.Now()))
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *locationRepo) toAvailabilityRecord(rec *core.Record, a *scheduling.Availability) {
	rec.Set("id", a.ID)
	rec.Set("location_id", a.LocationID)
	rec.Set("weekday", a.Weekday)
	rec.Set("start_time", a.StartTime)
	rec.Set("end_time", a.EndTime)
	rec.Set("capacity", a.Capacity)
	rec.Set("created_at", formatTime(a.CreatedAt))
	rec.Set("updated_at", formatTime(a.UpdatedAt))
}

func availabilityFromRecord(rec *core.Record) *scheduling.Availability {
	return &scheduling.Availability{
		ID:         rec.Id,
		LocationID: rec.GetString("location_id"),
		Weekday:    rec.GetInt("weekday"),
		StartTime:  rec.GetString("start_time"),
		EndTime:    rec.GetString("end_time"),
		Capacity:   rec.GetInt("capacity"),
		CreatedAt:  parseTime(rec.GetString("created_at")),
		UpdatedAt:  parseTime(rec.GetString("updated_at")),
	}
}

func (r *locationRepo) AddAvailability(ctx context.Context, a *scheduling.Availability) error {
	col, err := r.app.FindCollectionByNameOrId("availability")
	if err != nil {
		return mapError(err)
	}
	rec := core.NewRecord(col)
	r.toAvailabilityRecord(rec, a)
	return mapError(r.app.SaveWithContext(ctx, rec))
}

func (r *locationRepo) ListAvailability(ctx context.Context, locationID string) ([]*scheduling.Availability, error) {
	recs, err := r.app.FindRecordsByFilter("availability", "location_id = {:loc}", "weekday", 0, 0, dbxParams("loc", locationID))
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*scheduling.Availability, 0, len(recs))
	for _, rec := range recs {
		out = append(out, availabilityFromRecord(rec))
	}
	return out, nil
}

func (r *locationRepo) DeleteAvailability(ctx context.Context, id string) error {
	rec, err := getRecordByID(r.app, "availability", id)
	if err != nil {
		return err
	}
	return mapError(r.app.DeleteWithContext(ctx, rec))
}
