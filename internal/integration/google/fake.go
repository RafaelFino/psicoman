package google

import (
	"context"
	"sync"
)

// FakeCalendarClient é um CalendarClient em memória para testes — NENHUM teste
// toca a rede/API real do Google (psicoman-google-api.md). Configurável para
// acusar conflito no freebusy e devolver EventID/MeetURL determinísticos, e
// registra as chamadas para asserção.
//
// Exportado para reuso no wiring/API além dos testes do próprio pacote.
type FakeCalendarClient struct {
	mu sync.Mutex

	// Busy controla o retorno de FreeBusy (true = conflito).
	Busy bool
	// FreeBusyErr, se não-nil, é devolvido por FreeBusy.
	FreeBusyErr error

	// EventID / MeetURL compõem o CalendarEventOutput de CreateEvent.
	EventID string
	MeetURL string
	// CreateErr, se não-nil, é devolvido por CreateEvent.
	CreateErr error

	// FreeBusyCalls / CreateCalls registram as chamadas recebidas (asserção).
	FreeBusyCalls []CalendarFreeBusy
	CreateCalls   []CalendarEventInput
}

// garante que o fake satisfaz a interface do cliente real.
var _ CalendarClient = (*FakeCalendarClient)(nil)

// FreeBusy registra a consulta e devolve o conflito configurado.
func (f *FakeCalendarClient) FreeBusy(_ context.Context, q CalendarFreeBusy) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FreeBusyCalls = append(f.FreeBusyCalls, q)
	if f.FreeBusyErr != nil {
		return false, f.FreeBusyErr
	}
	return f.Busy, nil
}

// CreateEvent registra o input e devolve o resultado determinístico.
func (f *FakeCalendarClient) CreateEvent(_ context.Context, in CalendarEventInput) (*CalendarEventOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CreateCalls = append(f.CreateCalls, in)
	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	return &CalendarEventOutput{EventID: f.EventID, MeetURL: f.MeetURL}, nil
}
