// Package google implementa as portas de integração definidas em
// internal/service (CalendarClient, GmailClient, DriveClient) sobre as APIs
// REST do Google, além de fakes para testes (psicoman-google-api.md).
//
// A implementação real usa OAuth 3-legged com refresh token cifrado e chamadas
// REST diretas (sem o SDK pesado), sempre com timeout via context.
package google

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RafaelFino/psicoman/internal/service"
)

const (
	calendarBase = "https://www.googleapis.com/calendar/v3"
	gmailBase    = "https://gmail.googleapis.com/gmail/v1"
	driveBase    = "https://www.googleapis.com/drive/v3"
	uploadBase   = "https://www.googleapis.com/upload/drive/v3"
)

// TokenSource fornece um access token válido (renovando via refresh token).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Client implementa os três clientes Google sobre um TokenSource.
type Client struct {
	http       *http.Client
	tokens     TokenSource
	calendarID string
}

// NewClient cria o cliente real.
func NewClient(tokens TokenSource, calendarID string) *Client {
	if calendarID == "" {
		calendarID = "primary"
	}
	return &Client{
		http:       &http.Client{Timeout: 20 * time.Second},
		tokens:     tokens,
		calendarID: calendarID,
	}
}

var (
	_ service.CalendarClient = (*Client)(nil)
	_ service.GmailClient    = (*Client)(nil)
	_ service.DriveClient    = (*Client)(nil)
)

// do executa uma requisição autenticada e devolve o corpo se status 2xx.
func (c *Client) do(ctx context.Context, method, url string, body any, headers map[string]string) ([]byte, error) {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("google: obtendo token: %w", err)
	}
	var reader io.Reader
	if body != nil {
		if raw, ok := body.([]byte); ok {
			reader = bytes.NewReader(raw)
		} else {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reader = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if _, isRaw := body.([]byte); body != nil && !isRaw {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google: %s %s → %d: %s", method, url, resp.StatusCode, string(data))
	}
	return data, nil
}

// --- Calendar ---

// FreeBusy consulta o freebusy do calendário no intervalo.
func (c *Client) FreeBusy(ctx context.Context, startsAt, endsAt time.Time) (bool, error) {
	body := map[string]any{
		"timeMin": startsAt.Format(time.RFC3339),
		"timeMax": endsAt.Format(time.RFC3339),
		"items":   []map[string]string{{"id": c.calendarID}},
	}
	data, err := c.do(ctx, http.MethodPost, calendarBase+"/freeBusy", body, nil)
	if err != nil {
		return false, err
	}
	var out struct {
		Calendars map[string]struct {
			Busy []struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"busy"`
		} `json:"calendars"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, err
	}
	cal, ok := out.Calendars[c.calendarID]
	if !ok {
		return false, nil
	}
	return len(cal.Busy) > 0, nil
}

// CreateEvent cria um evento com convidado, Meet e reminders.
func (c *Client) CreateEvent(ctx context.Context, ev service.CalendarEvent) (*service.CalendarEventResult, error) {
	overrides := make([]map[string]any, 0, len(ev.ReminderMinutes))
	for _, m := range ev.ReminderMinutes {
		overrides = append(overrides, map[string]any{"method": "popup", "minutes": m})
	}
	payload := map[string]any{
		"summary":     ev.Summary,
		"description": ev.Description,
		"start":       map[string]string{"dateTime": ev.StartsAt.Format(time.RFC3339)},
		"end":         map[string]string{"dateTime": ev.EndsAt.Format(time.RFC3339)},
		"reminders":   map[string]any{"useDefault": false, "overrides": overrides},
	}
	if ev.AttendeeEmail != "" {
		payload["attendees"] = []map[string]string{{"email": ev.AttendeeEmail}}
	}
	url := calendarBase + "/calendars/" + c.calendarID + "/events?sendUpdates=all"
	if ev.WithMeet {
		payload["conferenceData"] = map[string]any{
			"createRequest": map[string]any{
				"requestId":             fmt.Sprintf("psicoman-%d", time.Now().UnixNano()),
				"conferenceSolutionKey": map[string]string{"type": "hangoutsMeet"},
			},
		}
		url += "&conferenceDataVersion=1"
	}
	data, err := c.do(ctx, http.MethodPost, url, payload, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		ID             string `json:"id"`
		HangoutLink    string `json:"hangoutLink"`
		ConferenceData struct {
			EntryPoints []struct {
				URI string `json:"uri"`
			} `json:"entryPoints"`
		} `json:"conferenceData"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	meet := out.HangoutLink
	if meet == "" && len(out.ConferenceData.EntryPoints) > 0 {
		meet = out.ConferenceData.EntryPoints[0].URI
	}
	return &service.CalendarEventResult{EventID: out.ID, MeetURL: meet}, nil
}

// DeleteEvent remove um evento.
func (c *Client) DeleteEvent(ctx context.Context, eventID string) error {
	_, err := c.do(ctx, http.MethodDelete, calendarBase+"/calendars/"+c.calendarID+"/events/"+eventID, nil, nil)
	return err
}

// --- Gmail ---

// Send envia um email HTML via Gmail API (mensagem RFC 2822 em base64url).
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	var msg strings.Builder
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	msg.WriteString(htmlBody)

	raw := base64.URLEncoding.EncodeToString([]byte(msg.String()))
	_, err := c.do(ctx, http.MethodPost, gmailBase+"/users/me/messages/send",
		map[string]string{"raw": raw}, nil)
	return err
}

// --- Drive ---

// Upload envia um arquivo ao Drive (multipart simples).
func (c *Client) Upload(ctx context.Context, folder string, file service.DriveFile) (string, error) {
	meta := map[string]any{"name": file.Name}
	if folder != "" {
		meta["parents"] = []string{folder}
	}
	metaJSON, _ := json.Marshal(meta)

	boundary := "psicoman-boundary"
	var body bytes.Buffer
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: application/json; charset=UTF-8\r\n\r\n")
	body.Write(metaJSON)
	body.WriteString("\r\n--" + boundary + "\r\n")
	body.WriteString("Content-Type: " + file.MIME + "\r\n\r\n")
	body.Write(file.Content)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	headers := map[string]string{"Content-Type": "multipart/related; boundary=" + boundary}
	data, err := c.do(ctx, http.MethodPost, uploadBase+"/files?uploadType=multipart", body.Bytes(), headers)
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// Download baixa o conteúdo de um arquivo do Drive.
func (c *Client) Download(ctx context.Context, fileID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, driveBase+"/files/"+fileID+"?alt=media", nil, nil)
}

// List lista arquivos de uma pasta (name→id).
func (c *Client) List(ctx context.Context, folder string) (map[string]string, error) {
	q := "trashed=false"
	if folder != "" {
		q = fmt.Sprintf("'%s' in parents and trashed=false", folder)
	}
	url := driveBase + "/files?q=" + q + "&fields=files(id,name)"
	data, err := c.do(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	res := map[string]string{}
	for _, f := range out.Files {
		res[f.Name] = f.ID
	}
	return res, nil
}
