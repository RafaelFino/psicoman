package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 0010_patient_scheduling cria o grupo A do MER canônico (D4b): as coleções de
// paciente e agendamento. Ordem respeita as dependências de relation
// (CollectionId exige o alvo já existente):
//
//	origin -> location -> patient -> availability -> appointment_request -> session
//
// session NÃO ganha contract_id/plan_id aqui — essas relations são adicionadas
// no grupo B (0020), após contract/plan existirem.
func init() {
	m.Register(upPatientScheduling, downPatientScheduling, "0010_patient_scheduling")
}

func upPatientScheduling(app core.App) error {
	// origin ---------------------------------------------------------------
	origin := newBaseCollection("origin")
	origin.Fields.Add(textField("name", true))
	timestamps(origin)
	softDelete(origin)
	origin.AddIndex("idx_origin_deleted_at", false, "deleted_at", "")
	if err := app.Save(origin); err != nil {
		return err
	}

	// location -------------------------------------------------------------
	location := newBaseCollection("location")
	location.Fields.Add(textField("name", true))
	location.Fields.Add(textField("address", false))
	location.Fields.Add(selectField("modality", false, "presencial", "online"))
	location.Fields.Add(numberIntField("cost_amount"))
	location.Fields.Add(selectField("cost_period", false, "por_sessao", "diario", "mensal", "anual"))
	timestamps(location)
	softDelete(location)
	location.AddIndex("idx_location_deleted_at", false, "deleted_at", "")
	if err := app.Save(location); err != nil {
		return err
	}

	// patient --------------------------------------------------------------
	patient := newBaseCollection("patient")
	patient.Fields.Add(textField("name", true))
	patient.Fields.Add(textField("phone", false))
	patient.Fields.Add(textField("email", true))
	patient.Fields.Add(textField("cpf", false))
	originRel, err := relationTo(app, "origin_id", "origin", false)
	if err != nil {
		return err
	}
	patient.Fields.Add(originRel)
	patient.Fields.Add(selectField("approval_status", false, "pendente", "aprovado", "rejeitado"))
	timestamps(patient)
	softDelete(patient)
	patient.AddIndex("ux_patient_email", true, "email", "")
	patient.AddIndex("ux_patient_cpf", true, "cpf", "cpf != '' AND cpf IS NOT NULL")
	patient.AddIndex("idx_patient_deleted_at", false, "deleted_at", "")
	if err := app.Save(patient); err != nil {
		return err
	}

	// availability ---------------------------------------------------------
	availability := newBaseCollection("availability")
	locRel, err := relationTo(app, "location_id", "location", true)
	if err != nil {
		return err
	}
	availability.Fields.Add(locRel)
	availability.Fields.Add(numberIntField("weekday"))
	availability.Fields.Add(textField("start_time", false))
	availability.Fields.Add(textField("end_time", false))
	availability.Fields.Add(numberIntField("capacity"))
	timestamps(availability)
	if err := app.Save(availability); err != nil {
		return err
	}

	// appointment_request --------------------------------------------------
	apptReq := newBaseCollection("appointment_request")
	arPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	apptReq.Fields.Add(arPatientRel)
	arLocationRel, err := relationTo(app, "location_id", "location", false)
	if err != nil {
		return err
	}
	apptReq.Fields.Add(arLocationRel)
	apptReq.Fields.Add(isoTextField("slot_start"))
	apptReq.Fields.Add(isoTextField("slot_end"))
	apptReq.Fields.Add(selectField("status", false, "pendente", "confirmado", "recusado"))
	apptReq.Fields.Add(textField("note", false))
	timestamps(apptReq)
	if err := app.Save(apptReq); err != nil {
		return err
	}

	// session --------------------------------------------------------------
	session := newBaseCollection("session")
	sPatientRel, err := relationTo(app, "patient_id", "patient", true)
	if err != nil {
		return err
	}
	session.Fields.Add(sPatientRel)
	sLocationRel, err := relationTo(app, "location_id", "location", false)
	if err != nil {
		return err
	}
	session.Fields.Add(sLocationRel)
	sRequestRel, err := relationTo(app, "request_id", "appointment_request", false)
	if err != nil {
		return err
	}
	session.Fields.Add(sRequestRel)
	session.Fields.Add(selectField("modality", false, "presencial", "online"))
	session.Fields.Add(isoTextField("starts_at"))
	session.Fields.Add(isoTextField("ends_at"))
	session.Fields.Add(selectField("status", false,
		"solicitada", "agendada", "confirmada", "realizada", "cancelada", "falta"))
	session.Fields.Add(boolField("bill"))
	session.Fields.Add(boolField("consider_cost"))
	session.Fields.Add(textField("google_event_id", false))
	session.Fields.Add(textField("meet_url", false))
	session.Fields.Add(isoTextField("confirmed_at"))
	session.Fields.Add(isoTextField("canceled_at"))
	session.Fields.Add(selectField("canceled_by", false, "paciente", "terapeuta"))
	session.Fields.Add(textField("cancel_reason", false))
	timestamps(session)
	softDelete(session)
	session.AddIndex("idx_session_deleted_at", false, "deleted_at", "")
	if err := app.Save(session); err != nil {
		return err
	}

	return nil
}

func downPatientScheduling(app core.App) error {
	// ordem inversa à criação (respeita as relations).
	return dropCollections(app,
		"session",
		"appointment_request",
		"availability",
		"patient",
		"location",
		"origin",
	)
}
