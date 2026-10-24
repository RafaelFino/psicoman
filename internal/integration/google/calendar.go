package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RafaelFino/psicoman/internal/app"
)

// CalendarScope é o escopo OAuth MÍNIMO exigido para Calendar/Meet
// (psicoman-google-api.md): só calendar, nunca mais amplo.
const CalendarScope = "https://www.googleapis.com/auth/calendar"

// calendarBaseURL é a base da Google Calendar API v3. Variável de pacote para
// permitir apontar ao servidor fake nos testes (sem tocar a rede real).
var calendarBaseURL = "https://www.googleapis.com/calendar/v3"

// CalendarFreeBusy descreve a janela consultada no freebusy do terapeuta.
type CalendarFreeBusy struct {
	CalendarID string
	Start      time.Time
	End        time.Time
}

// CalendarEventInput descreve o evento a criar no Google Calendar (incl. Meet,
// convidado e reminders). Espelha, sem acoplar, o app.CalendarEvent.
type CalendarEventInput struct {
	CalendarID      string
	Summary         string
	Description     string
	StartsAt        time.Time
	EndsAt          time.Time
	AttendeeEmail   string
	ReminderMinutes []int
	WithMeet        bool
}

// CalendarEventOutput é o resultado da criação do evento (id + link do Meet).
type CalendarEventOutput struct {
	EventID string
	MeetURL string
}

// CalendarClient é a interface LOCAL do cliente de Calendar (psicoman-google-api.md:
// toda integração Google vive atrás de uma interface, com fake nos testes —
// nenhum serviço chama a lib do Google direto). O adapter abaixo a implementa
// sobre a REST API v3; os testes usam um fake que satisfaz esta mesma interface.
//
// Todo método recebe context.Context e respeita o ctx (o chamador deriva o
// timeout; o adapter não cria timeout próprio — psicoman-golang.md).
type CalendarClient interface {
	// FreeBusy consulta a ocupação do calendário na janela e devolve true se há
	// qualquer intervalo ocupado (conflito), cobrindo também eventos externos.
	FreeBusy(ctx context.Context, q CalendarFreeBusy) (busy bool, err error)
	// CreateEvent cria o evento (com Meet quando WithMeet) e devolve id + MeetURL.
	CreateEvent(ctx context.Context, in CalendarEventInput) (*CalendarEventOutput, error)
}

// httpDoer é o transporte HTTP injetável (satisfeito por *http.Client e pelo
// cliente OAuth2). Permite trocar o transporte nos testes sem tocar a rede real.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// restCalendarClient fala com a Google Calendar API v3 sobre um httpDoer. O
// httpDoer carrega a autenticação (ex.: *http.Client do oauth2); este cliente
// só monta/parsa as requisições. Respeita o ctx recebido em toda chamada.
type restCalendarClient struct {
	http    httpDoer
	baseURL string
}

// NewRESTCalendarClient cria o cliente REST. O doer DEVE carregar a credencial
// OAuth (escopo calendar); nil não é aceito pelo wiring (degrada antes).
func NewRESTCalendarClient(doer httpDoer) CalendarClient {
	return &restCalendarClient{http: doer, baseURL: calendarBaseURL}
}

// FreeBusy consulta POST /freeBusy e reporta se há qualquer janela ocupada.
func (c *restCalendarClient) FreeBusy(ctx context.Context, q CalendarFreeBusy) (bool, error) {
	body := map[string]any{
		"timeMin": q.Start.Format(time.RFC3339),
		"timeMax": q.End.Format(time.RFC3339),
		"items":   []map[string]string{{"id": q.CalendarID}},
	}
	var out struct {
		Calendars map[string]struct {
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"calendars"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.baseURL+"/freeBusy", body, &out); err != nil {
		return false, err
	}
	for _, cal := range out.Calendars {
		if len(cal.Errors) > 0 {
			return false, fmt.Errorf("freebusy: calendário indisponível (%s)", cal.Errors[0].Reason)
		}
		if len(cal.Busy) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// CreateEvent cria o evento via POST /calendars/{id}/events. Quando WithMeet,
// pede a criação de conferência (Meet) via conferenceData + conferenceDataVersion=1.
func (c *restCalendarClient) CreateEvent(ctx context.Context, in CalendarEventInput) (*CalendarEventOutput, error) {
	event := map[string]any{
		"summary":     in.Summary,
		"description": in.Description,
		"start":       map[string]string{"dateTime": in.StartsAt.Format(time.RFC3339)},
		"end":         map[string]string{"dateTime": in.EndsAt.Format(time.RFC3339)},
	}
	if in.AttendeeEmail != "" {
		event["attendees"] = []map[string]string{{"email": in.AttendeeEmail}}
	}
	if len(in.ReminderMinutes) > 0 {
		overrides := make([]map[string]any, 0, len(in.ReminderMinutes))
		for _, m := range in.ReminderMinutes {
			overrides = append(overrides, map[string]any{"method": "popup", "minutes": m})
		}
		event["reminders"] = map[string]any{"useDefault": false, "overrides": overrides}
	}
	endpoint := c.baseURL + "/calendars/" + url.PathEscape(in.CalendarID) + "/events"
	if in.WithMeet {
		// conferenceDataVersion=1 habilita a criação de Meet via createRequest.
		event["conferenceData"] = map[string]any{
			"createRequest": map[string]any{
				"requestId":             "psicoman-" + in.StartsAt.Format("20060102T150405"),
				"conferenceSolutionKey": map[string]string{"type": "hangoutsMeet"},
			},
		}
		endpoint += "?conferenceDataVersion=1"
	}

	var out struct {
		ID             string `json:"id"`
		HangoutLink    string `json:"hangoutLink"`
		ConferenceData struct {
			EntryPoints []struct {
				EntryPointType string `json:"entryPointType"`
				URI            string `json:"uri"`
			} `json:"entryPoints"`
		} `json:"conferenceData"`
	}
	if err := c.doJSON(ctx, http.MethodPost, endpoint, event, &out); err != nil {
		return nil, err
	}
	meet := out.HangoutLink
	if meet == "" {
		for _, ep := range out.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				meet = ep.URI
				break
			}
		}
	}
	return &CalendarEventOutput{EventID: out.ID, MeetURL: meet}, nil
}

// doJSON executa uma requisição JSON respeitando o ctx recebido e decodifica a
// resposta em out. Status >= 400 vira erro (sem vazar o corpo bruto ao chamador).
func (c *restCalendarClient) doJSON(ctx context.Context, method, endpoint string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("calendar: serializar corpo: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("calendar: montar requisição: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calendar: chamada ao Google: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("calendar: Google respondeu %d", resp.StatusCode)
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("calendar: decodificar resposta: %w", err)
		}
	}
	return nil
}

// CalendarAdapter adapta um CalendarClient às portas app.ConflictChecker e
// app.SessionCalendar consumidas pelo SessionService. É o único Google no
// caminho de request (freebusy antes de confirmar; evento + Meet ao agendar).
type CalendarAdapter struct {
	client     CalendarClient
	calendarID string
}

// garante em tempo de compilação que o adapter satisfaz as duas portas do app.
var (
	_ app.ConflictChecker = (*CalendarAdapter)(nil)
	_ app.SessionCalendar = (*CalendarAdapter)(nil)
)

// NewCalendarAdapter cria o adapter sobre um CalendarClient e o calendário
// alvo (google.calendar_id). calendarID vazio cai para "primary".
func NewCalendarAdapter(client CalendarClient, calendarID string) *CalendarAdapter {
	id := strings.TrimSpace(calendarID)
	if id == "" {
		id = "primary"
	}
	return &CalendarAdapter{client: client, calendarID: id}
}

// HasConflict implementa app.ConflictChecker: consulta freebusy do terapeuta no
// intervalo. Respeita o ctx recebido (sem timeout próprio).
func (a *CalendarAdapter) HasConflict(ctx context.Context, startsAt, endsAt time.Time) (bool, error) {
	return a.client.FreeBusy(ctx, CalendarFreeBusy{
		CalendarID: a.calendarID,
		Start:      startsAt,
		End:        endsAt,
	})
}

// CreateEvent implementa app.SessionCalendar: cria o evento com convidado,
// reminders e Meet (quando WithMeet). Respeita o ctx recebido.
func (a *CalendarAdapter) CreateEvent(ctx context.Context, event app.CalendarEvent) (*app.CalendarEventResult, error) {
	out, err := a.client.CreateEvent(ctx, CalendarEventInput{
		CalendarID:      a.calendarID,
		Summary:         event.Summary,
		Description:     event.Description,
		StartsAt:        event.StartsAt,
		EndsAt:          event.EndsAt,
		AttendeeEmail:   event.AttendeeEmail,
		ReminderMinutes: event.ReminderMinutes,
		WithMeet:        event.WithMeet,
	})
	if err != nil {
		return nil, err
	}
	return &app.CalendarEventResult{EventID: out.EventID, MeetURL: out.MeetURL}, nil
}
