package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/RafaelFino/psicoman/internal/app"
)

// conflictingChecker acusa SEMPRE conflito, simulando o freebusy do Google
// retornando ocupado. NENHUM teste toca a rede/API real do Google.
type conflictingChecker struct{}

func (conflictingChecker) HasConflict(context.Context, time.Time, time.Time) (bool, error) {
	return true, nil
}

// withConflictingCalendar reatribui o seam calendarWirer para injetar um
// ConflictChecker que sempre acusa conflito, sem credenciais nem rede.
func withConflictingCalendar(t *testing.T) {
	t.Helper()
	prev := calendarWirer
	calendarWirer = func(_ core.App, s *app.SessionService) {
		s.SetConflictChecker(conflictingChecker{})
	}
	t.Cleanup(func() { calendarWirer = prev })
}

// TestSessionConflitoVira409 (FEAT-002): o conflito do Calendar (freebusy) ao
// agendar vira HTTP 409 com code 'conflito' no envelope PT-BR.
func TestSessionConflitoVira409(t *testing.T) {
	withConflictingCalendar(t)
	app := newTestApp(t)
	c := newClient(t, app)

	res := c.do(http.MethodPost, "/v1/app/patients",
		`{"name":"Rafa","phone":"11999990009","email":"rafa@exemplo.com"}`, appHeaders())
	if res.status != http.StatusCreated {
		t.Fatalf("criar paciente: status %d (%s)", res.status, res.body)
	}
	pid := extractID(res.body)
	if pid == "" {
		t.Fatalf("id do paciente não extraído: %s", res.body)
	}

	sres := c.do(http.MethodPost, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-10-21T14:00:00-03:00","ends_at":"2026-10-21T15:00:00-03:00"}`, appHeaders())
	if sres.status != http.StatusConflict {
		t.Fatalf("conflito de agenda: quero 409, veio %d (%s)", sres.status, sres.body)
	}
	if !strings.Contains(sres.body, `"code":"conflito"`) {
		t.Errorf("code esperado conflito no envelope: %s", sres.body)
	}
}

// TestSessionSemCalendarAgenda201 (FEAT-002): sem Calendar injetado (seam
// default sem credenciais no teste), a sessão agenda 201 sem evento e sem panic.
func TestSessionSemCalendarAgenda201(t *testing.T) {
	app := newTestApp(t)
	c := newClient(t, app)

	res := c.do(http.MethodPost, "/v1/app/patients",
		`{"name":"Sem Cal","phone":"11999990010","email":"semcal@exemplo.com"}`, appHeaders())
	pid := extractID(res.body)
	if pid == "" {
		t.Fatalf("id do paciente não extraído: %s", res.body)
	}
	sres := c.do(http.MethodPost, "/v1/app/sessions",
		`{"patient_id":"`+pid+`","modality":"online","starts_at":"2026-11-21T14:00:00-03:00","ends_at":"2026-11-21T15:00:00-03:00"}`, appHeaders())
	if sres.status != http.StatusCreated {
		t.Fatalf("sem Calendar: quero 201, veio %d (%s)", sres.status, sres.body)
	}
	// Sem Calendar, não há meet_url/google_event_id preenchidos.
	if strings.Contains(sres.body, `"meet_url":"https`) {
		t.Errorf("sem Calendar não deveria vir MeetURL: %s", sres.body)
	}
}
