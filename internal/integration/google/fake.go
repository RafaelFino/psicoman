package google

import (
	"context"
	"sync"
	"time"

	"github.com/RafaelFino/psicoman/internal/service"
)

// FakeClient é um cliente Google em memória para testes E2E (nenhuma rede).
// Implementa CalendarClient, GmailClient e DriveClient.
type FakeClient struct {
	mu sync.Mutex

	// Busy: intervalos ocupados para simular conflito de freebusy.
	Busy []Interval
	// Events criados (por id).
	Events map[string]service.CalendarEvent
	// SentEmails registra os envios.
	SentEmails []SentEmail
	// Drive: fileID → conteúdo; nome → fileID.
	driveContent map[string][]byte
	driveNames   map[string]string

	seq int
}

// Interval é um intervalo de tempo ocupado.
type Interval struct {
	Start time.Time
	End   time.Time
}

// SentEmail registra um email enviado no fake.
type SentEmail struct {
	To      string
	Subject string
	HTML    string
}

// NewFakeClient cria um fake vazio.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		Events:       map[string]service.CalendarEvent{},
		driveContent: map[string][]byte{},
		driveNames:   map[string]string{},
	}
}

var (
	_ service.CalendarClient = (*FakeClient)(nil)
	_ service.GmailClient    = (*FakeClient)(nil)
	_ service.DriveClient    = (*FakeClient)(nil)
)

// SetBusy marca um intervalo como ocupado (simula conflito).
func (f *FakeClient) SetBusy(start, end time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Busy = append(f.Busy, Interval{Start: start, End: end})
}

// FreeBusy devolve true se o intervalo se sobrepõe a algum ocupado.
func (f *FakeClient) FreeBusy(_ context.Context, startsAt, endsAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.Busy {
		if startsAt.Before(b.End) && b.Start.Before(endsAt) {
			return true, nil
		}
	}
	return false, nil
}

// CreateEvent registra um evento e devolve id + meet fake.
func (f *FakeClient) CreateEvent(_ context.Context, ev service.CalendarEvent) (*service.CalendarEventResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := "fake-event-" + itoa(f.seq)
	f.Events[id] = ev
	meet := ""
	if ev.WithMeet {
		meet = "https://meet.google.com/fake-" + itoa(f.seq)
	}
	return &service.CalendarEventResult{EventID: id, MeetURL: meet}, nil
}

// DeleteEvent remove um evento do fake.
func (f *FakeClient) DeleteEvent(_ context.Context, eventID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Events, eventID)
	return nil
}

// Send registra um email enviado.
func (f *FakeClient) Send(_ context.Context, to, subject, htmlBody string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SentEmails = append(f.SentEmails, SentEmail{To: to, Subject: subject, HTML: htmlBody})
	return nil
}

// Upload guarda o arquivo no drive fake.
func (f *FakeClient) Upload(_ context.Context, _ string, file service.DriveFile) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := "fake-drive-" + itoa(f.seq)
	f.driveContent[id] = append([]byte(nil), file.Content...)
	f.driveNames[file.Name] = id
	return id, nil
}

// Download devolve o conteúdo de um arquivo do drive fake.
func (f *FakeClient) Download(_ context.Context, fileID string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.driveContent[fileID], nil
}

// List devolve nome→id dos arquivos do drive fake.
func (f *FakeClient) List(_ context.Context, _ string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for name, id := range f.driveNames {
		out[name] = id
	}
	return out, nil
}

// EmailCount devolve quantos emails foram enviados.
func (f *FakeClient) EmailCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.SentEmails)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
