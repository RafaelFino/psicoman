package google

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
)

// start/end helpers determinísticos.
func janela() (time.Time, time.Time) {
	start := time.Date(2026, 3, 10, 10, 0, 0, 0, time.FixedZone("-03", -3*60*60))
	return start, start.Add(time.Hour)
}

// newFakeServer sobe um httptest.Server (NÃO é a API real do Google) e aponta o
// cliente REST para ele via baseURL. Devolve o cliente e o server para cleanup.
func newFakeServer(t *testing.T, h http.HandlerFunc) (CalendarClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := &restCalendarClient{http: srv.Client(), baseURL: srv.URL}
	return c, srv
}

func TestRESTFreeBusy_Ocupado(t *testing.T) {
	c, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/freeBusy") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"calendars": map[string]any{
				"primary": map[string]any{
					"busy": []map[string]string{{"start": "x", "end": "y"}},
				},
			},
		})
	})
	start, end := janela()
	busy, err := c.FreeBusy(context.Background(), CalendarFreeBusy{CalendarID: "primary", Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if !busy {
		t.Fatal("esperava conflito (busy=true)")
	}
}

func TestRESTFreeBusy_Livre(t *testing.T) {
	c, _ := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"calendars": map[string]any{"primary": map[string]any{"busy": []any{}}},
		})
	})
	start, end := janela()
	busy, err := c.FreeBusy(context.Background(), CalendarFreeBusy{CalendarID: "primary", Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if busy {
		t.Fatal("esperava livre (busy=false)")
	}
}

func TestRESTCreateEvent_ComMeetEConvidado(t *testing.T) {
	var gotBody map[string]any
	var gotConfVersion string
	c, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotConfVersion = r.URL.Query().Get("conferenceDataVersion")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "evt-123",
			"hangoutLink": "https://meet.google.com/abc-defg-hij",
		})
	})
	start, end := janela()
	out, err := c.CreateEvent(context.Background(), CalendarEventInput{
		CalendarID:      "primary",
		Summary:         "Sessão",
		AttendeeEmail:   "paciente@exemplo.com",
		StartsAt:        start,
		EndsAt:          end,
		ReminderMinutes: []int{1440, 30},
		WithMeet:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.EventID != "evt-123" || out.MeetURL != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("resultado inesperado: %+v", out)
	}
	if gotConfVersion != "1" {
		t.Errorf("WithMeet deveria mandar conferenceDataVersion=1, veio %q", gotConfVersion)
	}
	if _, ok := gotBody["conferenceData"]; !ok {
		t.Error("corpo deveria conter conferenceData para Meet")
	}
	if _, ok := gotBody["attendees"]; !ok {
		t.Error("corpo deveria conter o paciente como convidado")
	}
	if _, ok := gotBody["reminders"]; !ok {
		t.Error("corpo deveria conter os reminders")
	}
}

func TestRESTCreateEvent_SemMeet(t *testing.T) {
	var gotConfVersion string
	var gotBody map[string]any
	c, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotConfVersion = r.URL.Query().Get("conferenceDataVersion")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt-sem-meet"})
	})
	start, end := janela()
	out, err := c.CreateEvent(context.Background(), CalendarEventInput{
		CalendarID: "primary", Summary: "Sessão presencial",
		StartsAt: start, EndsAt: end, WithMeet: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.MeetURL != "" {
		t.Errorf("sem Meet não deveria ter MeetURL, veio %q", out.MeetURL)
	}
	if gotConfVersion != "" {
		t.Errorf("sem Meet não deveria mandar conferenceDataVersion, veio %q", gotConfVersion)
	}
	if _, ok := gotBody["conferenceData"]; ok {
		t.Error("sem Meet não deveria conter conferenceData")
	}
}

func TestRESTDoJSON_ErroHTTP(t *testing.T) {
	c, _ := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})
	start, end := janela()
	if _, err := c.FreeBusy(context.Background(), CalendarFreeBusy{CalendarID: "primary", Start: start, End: end}); err == nil {
		t.Fatal("status 403 deveria virar erro")
	}
}

func TestRESTRespeitaContext(t *testing.T) {
	c, _ := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start, end := janela()
	if _, err := c.FreeBusy(ctx, CalendarFreeBusy{CalendarID: "primary", Start: start, End: end}); err == nil {
		t.Fatal("ctx expirado deveria abortar a chamada")
	}
}

// --- adapter sobre o fake (satisfaz as portas do app) ---

func TestAdapter_HasConflict_ViaFreeBusy(t *testing.T) {
	fake := &FakeCalendarClient{Busy: true}
	adp := NewCalendarAdapter(fake, "cal-do-terapeuta")
	start, end := janela()
	busy, err := adp.HasConflict(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if !busy {
		t.Fatal("adapter deveria propagar conflito do freebusy")
	}
	if len(fake.FreeBusyCalls) != 1 || fake.FreeBusyCalls[0].CalendarID != "cal-do-terapeuta" {
		t.Fatalf("freebusy chamado errado: %+v", fake.FreeBusyCalls)
	}
}

func TestAdapter_CreateEvent_PropagaMeetEConvidado(t *testing.T) {
	fake := &FakeCalendarClient{EventID: "evt-x", MeetURL: "https://meet/x"}
	adp := NewCalendarAdapter(fake, "")
	start, end := janela()
	res, err := adp.CreateEvent(context.Background(), app.CalendarEvent{
		Summary: "Sessão", AttendeeEmail: "p@x.com",
		StartsAt: start, EndsAt: end, ReminderMinutes: []int{1440, 30}, WithMeet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EventID != "evt-x" || res.MeetURL != "https://meet/x" {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if len(fake.CreateCalls) != 1 {
		t.Fatalf("esperava 1 CreateEvent, veio %d", len(fake.CreateCalls))
	}
	call := fake.CreateCalls[0]
	if !call.WithMeet || call.AttendeeEmail != "p@x.com" || call.CalendarID != "primary" {
		t.Fatalf("input repassado errado: %+v", call)
	}
}

func TestAdapter_PropagaErro(t *testing.T) {
	boom := errors.New("falha google")
	adp := NewCalendarAdapter(&FakeCalendarClient{CreateErr: boom}, "primary")
	start, end := janela()
	if _, err := adp.CreateEvent(context.Background(), app.CalendarEvent{StartsAt: start, EndsAt: end}); !errors.Is(err, boom) {
		t.Fatalf("erro do cliente deveria propagar, veio %v", err)
	}
}
