package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/integration/google"
)

func TestScheduleCreatesCalendarEvent(t *testing.T) {
	env := StartAdmin(t)
	defer env.Stop()

	pid := env.createPatient(t, "calendar@example.com")
	start := time.Now().Add(24 * time.Hour)
	end := start.Add(time.Hour)

	// Cria como solicitada e agenda → deve criar evento + Meet no fake.
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start.Format(time.RFC3339), "ends_at": end.Format(time.RFC3339),
		"status": "solicitada",
	})
	se := DecodeEnvelope(t, resp)
	var s struct {
		ID string `json:"id"`
	}
	se.DataAs(t, &s)

	resp = env.AdminPOST(t, "/v1/admin/sessions/"+s.ID+"/schedule", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schedule status = %d, quer 200", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	var sched struct {
		Status  string `json:"status"`
		MeetURL string `json:"meet_url"`
	}
	e.DataAs(t, &sched)
	if sched.Status != "agendada" {
		t.Errorf("status = %q, quer agendada", sched.Status)
	}
	if sched.MeetURL == "" {
		t.Error("evento agendado sem link do Meet")
	}
	if len(env.Google.Events) != 1 {
		t.Errorf("eventos no Calendar = %d, quer 1", len(env.Google.Events))
	}
}

func TestScheduleBlockedByConflict(t *testing.T) {
	fake := google.NewFakeClient()
	// Marca o horário como ocupado (conflito de agenda).
	start := time.Now().Add(48 * time.Hour)
	end := start.Add(time.Hour)
	fake.SetBusy(start, end)

	env := StartAdminWithGoogle(t, fake)
	defer env.Stop()

	pid := env.createPatient(t, "conflito@example.com")

	// Criar direto como agendada no horário ocupado → 409.
	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start.Format(time.RFC3339), "ends_at": end.Format(time.RFC3339),
		"status": "agendada",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, quer 409 (conflito)", resp.StatusCode)
	}
	e := DecodeEnvelope(t, resp)
	if e.Message == "" {
		t.Error("conflito sem mensagem PT-BR")
	}
	// Nenhum evento criado.
	if len(env.Google.Events) != 0 {
		t.Errorf("eventos = %d, quer 0 (conflito bloqueia)", len(env.Google.Events))
	}
}

func TestScheduleWithoutConflictCreatesEvent(t *testing.T) {
	fake := google.NewFakeClient()
	// Ocupa um horário diferente do da sessão.
	busyStart := time.Now().Add(100 * time.Hour)
	fake.SetBusy(busyStart, busyStart.Add(time.Hour))

	env := StartAdminWithGoogle(t, fake)
	defer env.Stop()

	pid := env.createPatient(t, "livre@example.com")
	start := time.Now().Add(72 * time.Hour)
	end := start.Add(time.Hour)

	resp := env.AdminPOST(t, "/v1/admin/sessions", map[string]any{
		"patient_id": pid, "modality": "online",
		"starts_at": start.Format(time.RFC3339), "ends_at": end.Format(time.RFC3339),
		"status": "agendada",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, quer 201", resp.StatusCode)
	}
	if len(env.Google.Events) != 1 {
		t.Errorf("eventos = %d, quer 1", len(env.Google.Events))
	}
}
