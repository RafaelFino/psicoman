package service

import (
	"context"
	"fmt"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarClient interface {
	CreateEvent(ctx context.Context, appt domain.Appointment, patientName string) (eventID, meetLink string, err error)
	UpdateEvent(ctx context.Context, eventID string, appt domain.Appointment, patientName string) (meetLink string, err error)
	DeleteEvent(ctx context.Context, eventID string) error
}

type GoogleCalendar struct {
	clientID     string
	clientSecret string
	redirectURL  string
	calendarID   string
}

func NewGoogleCalendar(clientID, clientSecret, redirectURL, calendarID string) *GoogleCalendar {
	return &GoogleCalendar{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		calendarID:   calendarID,
	}
}

func (g *GoogleCalendar) OAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     g.clientID,
		ClientSecret: g.clientSecret,
		RedirectURL:  g.redirectURL,
		Scopes:       []string{calendar.CalendarScope},
		Endpoint:     google.Endpoint,
	}
}

func (g *GoogleCalendar) AuthURL(state string) string {
	return g.OAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (g *GoogleCalendar) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.OAuthConfig().Exchange(ctx, code)
}

func (g *GoogleCalendar) SaveToken(db *storage.DB, token *oauth2.Token) error {
	return db.SaveGoogleTokens(storage.GoogleTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	})
}

func (g *GoogleCalendar) service(ctx context.Context, db *storage.DB) (*calendar.Service, error) {
	tokens, err := db.GetGoogleTokens()
	if err != nil {
		return nil, fmt.Errorf("google não conectado: %w", err)
	}
	ts := g.OAuthConfig().TokenSource(ctx, &oauth2.Token{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Expiry:       tokens.Expiry,
	})
	return calendar.NewService(ctx, option.WithTokenSource(ts))
}

func (g *GoogleCalendar) CreateEvent(ctx context.Context, db *storage.DB, appt domain.Appointment, patientName string) (string, string, error) {
	svc, err := g.service(ctx, db)
	if err != nil {
		return g.fallback(appt)
	}

	end := appt.ScheduledAt.Add(time.Duration(appt.DurationMinutes) * time.Minute)
	event := &calendar.Event{
		Summary:     fmt.Sprintf("Consulta: %s", patientName),
		Description: fmt.Sprintf("Tipo: %s", appt.Type),
		Start:       &calendar.EventDateTime{DateTime: appt.ScheduledAt.Format(time.RFC3339)},
		End:         &calendar.EventDateTime{DateTime: end.Format(time.RFC3339)},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: appt.ID,
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
			},
		},
	}

	created, err := svc.Events.Insert(g.calendarID, event).ConferenceDataVersion(1).Context(ctx).Do()
	if err != nil {
		return g.fallback(appt)
	}

	meetLink := ""
	if created.HangoutLink != "" {
		meetLink = created.HangoutLink
	} else if created.ConferenceData != nil {
		for _, ep := range created.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				meetLink = ep.Uri
				break
			}
		}
	}
	return created.Id, meetLink, nil
}

func (g *GoogleCalendar) UpdateEvent(ctx context.Context, db *storage.DB, eventID string, appt domain.Appointment, patientName string) (string, error) {
	svc, err := g.service(ctx, db)
	if err != nil {
		return appt.MeetLink, err
	}
	end := appt.ScheduledAt.Add(time.Duration(appt.DurationMinutes) * time.Minute)
	event := &calendar.Event{
		Summary: fmt.Sprintf("Consulta: %s", patientName),
		Start:   &calendar.EventDateTime{DateTime: appt.ScheduledAt.Format(time.RFC3339)},
		End:     &calendar.EventDateTime{DateTime: end.Format(time.RFC3339)},
	}
	updated, err := svc.Events.Patch(g.calendarID, eventID, event).Context(ctx).Do()
	if err != nil {
		return appt.MeetLink, err
	}
	if updated.HangoutLink != "" {
		return updated.HangoutLink, nil
	}
	return appt.MeetLink, nil
}

func (g *GoogleCalendar) DeleteEvent(ctx context.Context, db *storage.DB, eventID string) error {
	if eventID == "" {
		return nil
	}
	svc, err := g.service(ctx, db)
	if err != nil {
		return err
	}
	return svc.Events.Delete(g.calendarID, eventID).Context(ctx).Do()
}

func (g *GoogleCalendar) fallback(appt domain.Appointment) (string, string, error) {
	link := fmt.Sprintf("https://meet.google.com/lookup/%s", appt.ID[:8])
	return "", link, nil
}

// NoopCalendar for tests or when Google is not configured.
type NoopCalendar struct{}

func (n *NoopCalendar) CreateEvent(_ context.Context, appt domain.Appointment, _ string) (string, string, error) {
	link := fmt.Sprintf("https://meet.google.com/lookup/%s", appt.ID[:8])
	return "", link, nil
}

func (n *NoopCalendar) UpdateEvent(_ context.Context, _ string, appt domain.Appointment, _ string) (string, error) {
	return appt.MeetLink, nil
}

func (n *NoopCalendar) DeleteEvent(_ context.Context, _ string) error { return nil }
