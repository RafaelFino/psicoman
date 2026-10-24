package api

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/platform/ulid"
)

// savePatient cria um patient diretamente (via app.Save) com o estado de
// aprovação informado e devolve o id. Escreve pela mesma via do adapter
// (model-level), sem passar pela API.
func savePatient(t *testing.T, app *tests.TestApp, email, status string) string {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("patient")
	if err != nil {
		t.Fatalf("coleção patient: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("id", ulid.New())
	rec.Set("name", "Paciente Hook")
	rec.Set("email", email)
	rec.Set("approval_status", status)
	if err := app.Save(rec); err != nil {
		t.Fatalf("salvar patient (%s): %v", status, err)
	}
	return rec.Id
}

// newAppointmentRequest monta (sem salvar) um appointment_request apontando para
// patientID.
func newAppointmentRequest(t *testing.T, app *tests.TestApp, patientID string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("appointment_request")
	if err != nil {
		t.Fatalf("coleção appointment_request: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("id", ulid.New())
	rec.Set("patient_id", patientID)
	rec.Set("status", "pendente")
	return rec
}

// TestHookBarraAgendamentoDePendente (5.3): escrita direta (app.Save) de
// appointment_request cujo dono está pendente é BARRADA pelo hook de reforço —
// prova defesa em profundidade independente do middleware.
func TestHookBarraAgendamentoDePendente(t *testing.T) {
	app := newTestApp(t)
	// Dispara o OnServe para registrar os hooks (registerApprovalHooks roda lá).
	newClient(t, app)

	pid := savePatient(t, app, "pend-hook@exemplo.com", patient.PatientPending)
	ar := newAppointmentRequest(t, app, pid)
	if err := app.Save(ar); err == nil {
		t.Fatalf("esperava erro ao agendar para paciente pendente, mas gravou")
	}
}

// TestHookPermiteAgendamentoDeAprovado (5.3): mesmo registro com dono aprovado →
// grava sem erro.
func TestHookPermiteAgendamentoDeAprovado(t *testing.T) {
	app := newTestApp(t)
	newClient(t, app)

	pid := savePatient(t, app, "aprov-hook@exemplo.com", patient.PatientApproved)
	ar := newAppointmentRequest(t, app, pid)
	if err := app.Save(ar); err != nil {
		t.Fatalf("esperava gravar para paciente aprovado, veio erro: %v", err)
	}
}

// TestHookInvarianteTerapeuta (5.3, Finding 2 Rev.1): o hook é cego ao ator —
// mesmo a escrita que o terapeuta faria (via adapter/app.Save) para paciente
// pendente é barrada. Fixa o invariante "agendar exige paciente aprovado, venha
// de quem vier".
func TestHookInvarianteTerapeuta(t *testing.T) {
	app := newTestApp(t)
	newClient(t, app)

	pid := savePatient(t, app, "terap-hook@exemplo.com", patient.PatientPending)
	ar := newAppointmentRequest(t, app, pid)
	if err := app.Save(ar); err == nil {
		t.Fatalf("invariante: agendar para pendente deveria ser barrado, mas gravou")
	}
}

// TestHookPatientForaDoHook (5.3, Finding 1 Rev.2): escrita direta de um patient
// pendente (upsert/edição/soft-delete) NÃO é barrada — nenhum hook de patient
// está ligado. Regressão da decisão de escopo (só appointment_request).
func TestHookPatientForaDoHook(t *testing.T) {
	app := newTestApp(t)
	newClient(t, app)

	// create de patient pendente não é barrado.
	pid := savePatient(t, app, "livre-hook@exemplo.com", patient.PatientPending)

	// update (edição) de patient mantendo pendente não é barrado.
	rec, err := app.FindRecordById("patient", pid)
	if err != nil {
		t.Fatalf("buscar patient: %v", err)
	}
	rec.Set("phone", "5511999990000")
	if err := app.Save(rec); err != nil {
		t.Fatalf("editar patient pendente não deveria ser barrado, veio: %v", err)
	}

	// soft-delete (grava deleted_at) de patient pendente não é barrado.
	rec.Set("deleted_at", "2026-10-21 12:00:00.000Z")
	if err := app.Save(rec); err != nil {
		t.Fatalf("soft-delete de patient pendente não deveria ser barrado, veio: %v", err)
	}
}
