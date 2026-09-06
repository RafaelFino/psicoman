package service

import (
	"context"
	"time"
)

// Portas de integração com o Google. Definidas na camada de serviço (não na
// integração) para permitir fakes nos testes (psicoman-google-api.md). A
// implementação real vive em internal/integration/google.

// CalendarEvent descreve um evento a criar no Google Calendar.
type CalendarEvent struct {
	Summary         string
	Description     string
	StartsAt        time.Time
	EndsAt          time.Time
	AttendeeEmail   string
	ReminderMinutes []int // lembretes acumulativos (minutos antes)
	WithMeet        bool
}

// CalendarEventResult é o resultado da criação de um evento.
type CalendarEventResult struct {
	EventID string
	MeetURL string
}

// CalendarClient acessa o Google Calendar/Meet.
type CalendarClient interface {
	// FreeBusy indica se há conflito no intervalo [startsAt, endsAt).
	FreeBusy(ctx context.Context, startsAt, endsAt time.Time) (busy bool, err error)
	CreateEvent(ctx context.Context, event CalendarEvent) (*CalendarEventResult, error)
	DeleteEvent(ctx context.Context, eventID string) error
}

// GmailClient envia emails (escopo gmail.send).
type GmailClient interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// DriveFile descreve um arquivo para upload no Drive.
type DriveFile struct {
	Name    string
	Content []byte
	MIME    string
}

// DriveClient acessa o Google Drive (escopo drive.file).
type DriveClient interface {
	Upload(ctx context.Context, folder string, file DriveFile) (fileID string, err error)
	Download(ctx context.Context, fileID string) ([]byte, error)
	List(ctx context.Context, folder string) (map[string]string, error) // name→fileID
}

// gmailTemplateSender adapta um GmailClient à porta TemplateSender do prontuário.
type gmailTemplateSender struct{ gmail GmailClient }

// NewGmailTemplateSender cria um TemplateSender que envia via Gmail.
func NewGmailTemplateSender(gmail GmailClient) TemplateSender {
	return gmailTemplateSender{gmail: gmail}
}

func (s gmailTemplateSender) Send(ctx context.Context, toEmail, subject, html string) error {
	return s.gmail.Send(ctx, toEmail, subject, html)
}
