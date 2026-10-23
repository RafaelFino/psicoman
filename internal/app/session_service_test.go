package app

import (
	"context"
	"testing"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
	"github.com/RafaelFino/psicoman/internal/platform/clock"
)

func newSessionSvc(repo SessionRepository, patients PatientRepository, c clock.Clock) *SessionService {
	s := NewSessionService(repo, patients)
	s.clock = c
	return s
}

func seedPatient(repo *fakePatientRepo, id string) {
	repo.byID[id] = &patient.Patient{ID: id, Name: "Fulano", Email: id + "@x.com"}
}

func createInput() SessionCreateInput {
	return SessionCreateInput{
		PatientID: "p1", LocationID: "loc1", Modality: scheduling.ModalityOnline,
		StartsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()),
		EndsAt:   time.Date(2026, 3, 10, 11, 0, 0, 0, clock.Location()),
		Status:   scheduling.SessionAgendada,
	}
}

func TestSessionCreate_AgendadaChecaConflitoECriaEvento(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 1))

	conf := &fakeConflictChecker{}
	svc.SetConflictChecker(conf)
	cal := &fakeCalendar{result: CalendarEventResult{EventID: "evt1", MeetURL: "https://meet/x"}}
	svc.SetCalendar(cal, []int{30})

	sess, err := svc.Create(ctx, createInput())
	if err != nil {
		t.Fatal(err)
	}
	if conf.calls != 1 {
		t.Fatalf("esperava checagem de conflito, calls=%d", conf.calls)
	}
	if cal.calls != 1 {
		t.Fatalf("esperava criação de evento, calls=%d", cal.calls)
	}
	if sess.GoogleEventID != "evt1" || sess.MeetURL != "https://meet/x" {
		t.Fatalf("evento não gravado na sessão: %+v", sess)
	}
	if repo.creates != 1 {
		t.Fatalf("sessão não persistida, creates=%d", repo.creates)
	}
}

func TestSessionCreate_ConflitoBloqueia(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 1))
	svc.SetConflictChecker(&fakeConflictChecker{conflict: true})

	_, err := svc.Create(ctx, createInput())
	if err == nil {
		t.Fatal("esperava conflito de agenda")
	}
	if repo.creates != 0 {
		t.Fatalf("não deveria persistir em conflito, creates=%d", repo.creates)
	}
}

func TestSessionFinish_DisparaHooks(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	seedPatient(patients, "p1")
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 10))
	hook := &recordingHook{}
	svc.AddFinishedHook(hook)

	// Sessão agendada no repo.
	repo.byID["s1"] = &scheduling.Session{
		ID: "s1", PatientID: "p1", Modality: scheduling.ModalityOnline, Status: scheduling.SessionAgendada,
		StartsAt: time.Date(2026, 3, 10, 10, 0, 0, 0, clock.Location()),
		EndsAt:   time.Date(2026, 3, 10, 11, 0, 0, 0, clock.Location()),
	}
	sess, err := svc.Finish(ctx, "s1", FinishInput{})
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != scheduling.SessionRealizada || !sess.Bill {
		t.Fatalf("finish padrão deveria marcar realizada+bill: %+v", sess)
	}
	if len(hook.finished) != 1 || hook.finished[0].ID != "s1" {
		t.Fatalf("hook não disparado corretamente: %+v", hook.finished)
	}
}

func TestSessionConfirm_TransicaoInvalida(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 10))
	audit, arepo := newAudit()
	svc.SetAudit(audit)

	// Sessão já realizada (terminal) não pode confirmar.
	repo.byID["s1"] = &scheduling.Session{ID: "s1", PatientID: "p1", Status: scheduling.SessionRealizada}
	if _, err := svc.Confirm(ctx, "s1"); err == nil {
		t.Fatal("esperava erro de transição inválida")
	}
	if len(arepo.entries) != 0 {
		t.Fatalf("transição inválida não deveria auditar, entries=%d", len(arepo.entries))
	}
}

func TestSessionCancel_AuditaAutor(t *testing.T) {
	ctx := context.Background()
	repo := newSessionRepo()
	patients := newPatientRepo()
	svc := newSessionSvc(repo, patients, fixedAt(2026, 3, 10))
	audit, arepo := newAudit()
	svc.SetAudit(audit)

	repo.byID["s1"] = &scheduling.Session{ID: "s1", PatientID: "p1", Status: scheduling.SessionAgendada}
	if _, err := svc.Cancel(ctx, "s1", scheduling.CancelByPatient, "imprevisto"); err != nil {
		t.Fatal(err)
	}
	acts := arepo.actionsFor("session", "s1")
	if len(acts) != 1 {
		t.Fatalf("esperava auditar o cancelamento, veio %v", acts)
	}
}
