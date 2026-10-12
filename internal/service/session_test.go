package service

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

// fakeSessionRepo é um SessionRepository em memória para os testes de serviço.
type fakeSessionRepo struct {
	byID map[string]*domain.Session
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{byID: map[string]*domain.Session{}}
}

func (f *fakeSessionRepo) put(s *domain.Session) {
	cp := *s
	f.byID[s.ID] = &cp
}

func (f *fakeSessionRepo) Create(_ context.Context, s *domain.Session) error {
	f.put(s)
	return nil
}

func (f *fakeSessionRepo) Update(_ context.Context, s *domain.Session) error {
	if _, ok := f.byID[s.ID]; !ok {
		return ErrNotFound
	}
	f.put(s)
	return nil
}

func (f *fakeSessionRepo) Get(_ context.Context, id string) (*domain.Session, error) {
	if s, ok := f.byID[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (f *fakeSessionRepo) List(_ context.Context) ([]*domain.Session, error) {
	out := make([]*domain.Session, 0, len(f.byID))
	for _, s := range f.byID {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeSessionRepo) ListByPatient(_ context.Context, patientID string) ([]*domain.Session, error) {
	var out []*domain.Session
	for _, s := range f.byID {
		if s.PatientID == patientID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepo) ListByRange(_ context.Context, from, to time.Time) ([]*domain.Session, error) {
	var out []*domain.Session
	for _, s := range f.byID {
		if !s.StartsAt.Before(from) && s.StartsAt.Before(to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

// newSessionSvcWith monta o serviço com um paciente já cadastrado e devolve
// também o repositório de sessões para inspeção.
func newSessionSvcWith(t *testing.T) (*SessionService, *fakeSessionRepo, string) {
	t.Helper()
	patients := newFakePatientRepo()
	p := &domain.Patient{ID: "pat-1", Name: "Ana", Email: "ana@x.com"}
	patients.put(p)
	repo := newFakeSessionRepo()
	svc := &SessionService{
		repo:     repo,
		patients: patients,
		conflict: noopConflictChecker{},
		clock:    clock.Fixed{T: time.Date(2026, 10, 12, 9, 0, 0, 0, clock.Location())},
	}
	return svc, repo, p.ID
}

func mustSession(t *testing.T, repo *fakeSessionRepo, status string) *domain.Session {
	t.Helper()
	start := time.Date(2026, 10, 13, 10, 0, 0, 0, clock.Location())
	s := &domain.Session{
		ID:        "sess-1",
		PatientID: "pat-1",
		Modality:  domain.ModalityPresencial,
		StartsAt:  start,
		EndsAt:    start.Add(time.Hour),
		Status:    status,
		CreatedAt: start,
		UpdatedAt: start,
	}
	repo.put(s)
	return s
}

func TestSessionUpdateSuccess(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionAgendada)

	newStart := time.Date(2026, 10, 14, 15, 0, 0, 0, clock.Location())
	got, err := svc.Update(context.Background(), "sess-1", SessionUpdateInput{
		LocationID: "loc-9",
		Modality:   domain.ModalityOnline,
		StartsAt:   newStart,
		EndsAt:     newStart.Add(50 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !got.StartsAt.Equal(newStart) {
		t.Errorf("StartsAt não aplicado: %v", got.StartsAt)
	}
	if got.Modality != domain.ModalityOnline {
		t.Errorf("modalidade esperada online, veio %q", got.Modality)
	}
	if got.LocationID != "loc-9" {
		t.Errorf("local esperado loc-9, veio %q", got.LocationID)
	}
	if got.PatientID != "pat-1" {
		t.Errorf("paciente não deveria mudar, veio %q", got.PatientID)
	}
	// Persistido no repo.
	stored, _ := repo.Get(context.Background(), "sess-1")
	if !stored.StartsAt.Equal(newStart) {
		t.Errorf("alteração não persistida")
	}
}

func TestSessionUpdateTerminalBlocked(t *testing.T) {
	for _, status := range []string{domain.SessionRealizada, domain.SessionCancelada, domain.SessionFalta} {
		t.Run(status, func(t *testing.T) {
			svc, repo, _ := newSessionSvcWith(t)
			mustSession(t, repo, status)
			newStart := time.Date(2026, 10, 14, 15, 0, 0, 0, clock.Location())
			_, err := svc.Update(context.Background(), "sess-1", SessionUpdateInput{
				Modality: domain.ModalityPresencial,
				StartsAt: newStart,
				EndsAt:   newStart.Add(time.Hour),
			})
			if err == nil {
				t.Fatalf("esperava erro ao editar sessão em estado terminal %q", status)
			}
		})
	}
}

func TestSessionUpdateEndBeforeStart(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionAgendada)
	start := time.Date(2026, 10, 14, 15, 0, 0, 0, clock.Location())
	_, err := svc.Update(context.Background(), "sess-1", SessionUpdateInput{
		Modality: domain.ModalityPresencial,
		StartsAt: start,
		EndsAt:   start.Add(-time.Hour), // fim antes do início
	})
	if err == nil {
		t.Fatal("esperava erro de validação (fim antes do início)")
	}
}

func TestSessionUpdateNotFound(t *testing.T) {
	svc, _, _ := newSessionSvcWith(t)
	start := time.Date(2026, 10, 14, 15, 0, 0, 0, clock.Location())
	_, err := svc.Update(context.Background(), "inexistente", SessionUpdateInput{
		Modality: domain.ModalityPresencial,
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	})
	if err == nil {
		t.Fatal("esperava ErrNotFound para sessão inexistente")
	}
}

func TestSessionConfirm(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionAgendada)

	got, err := svc.Confirm(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got.Status != domain.SessionConfirmada {
		t.Errorf("status = %q, quer confirmada", got.Status)
	}
	if got.ConfirmedAt.IsZero() {
		t.Error("confirmed_at não gravado")
	}
	// Persistido.
	stored, _ := repo.Get(context.Background(), "sess-1")
	if stored.Status != domain.SessionConfirmada || stored.ConfirmedAt.IsZero() {
		t.Error("confirmação não persistida")
	}
}

func TestSessionConfirmInvalidState(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionRealizada)
	if _, err := svc.Confirm(context.Background(), "sess-1"); err == nil {
		t.Fatal("confirmar sessão realizada deveria falhar")
	}
}

func TestSessionCancelRequiresAuthor(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionAgendada)

	// Sem autor → erro de validação.
	if _, err := svc.Cancel(context.Background(), "sess-1", "", "qualquer"); err == nil {
		t.Fatal("cancelar sem autor deveria falhar")
	}
	// Autor inválido → erro.
	if _, err := svc.Cancel(context.Background(), "sess-1", "sistema", ""); err == nil {
		t.Fatal("autor inválido deveria falhar")
	}
}

func TestSessionCancelWithMetadata(t *testing.T) {
	svc, repo, _ := newSessionSvcWith(t)
	mustSession(t, repo, domain.SessionConfirmada)

	got, err := svc.Cancel(context.Background(), "sess-1", domain.CancelByPatient, "imprevisto")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != domain.SessionCancelada {
		t.Errorf("status = %q, quer cancelada", got.Status)
	}
	if got.CanceledBy != domain.CancelByPatient {
		t.Errorf("canceled_by = %q, quer paciente", got.CanceledBy)
	}
	if got.CancelReason != "imprevisto" {
		t.Errorf("cancel_reason = %q, quer imprevisto", got.CancelReason)
	}
	if got.CanceledAt.IsZero() {
		t.Error("canceled_at não gravado")
	}
	// confirmada → cancelada é transição válida.
	stored, _ := repo.Get(context.Background(), "sess-1")
	if stored.CanceledBy != domain.CancelByPatient {
		t.Error("metadados de cancelamento não persistidos")
	}
}
