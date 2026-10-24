package app

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// Fakes em memória das portas, suficientes para exercitar a orquestração dos
// serviços de app. Não reimplementam regra de domínio; só guardam/devolvem.

// --- audit ---

type fakeAuditRepo struct {
	entries []*domain.AuditLog
}

func (f *fakeAuditRepo) Insert(_ context.Context, e *domain.AuditLog) error {
	f.entries = append(f.entries, e)
	return nil
}

func (f *fakeAuditRepo) List(_ context.Context, limit int) ([]*domain.AuditLog, error) {
	return f.entries, nil
}

func (f *fakeAuditRepo) ListByEntity(_ context.Context, entity, entityID string) ([]*domain.AuditLog, error) {
	var out []*domain.AuditLog
	for _, e := range f.entries {
		if e.Entity == entity && e.EntityID == entityID {
			out = append(out, e)
		}
	}
	return out, nil
}

// actionsFor devolve as ações registradas para uma entidade/id.
func (f *fakeAuditRepo) actionsFor(entity, entityID string) []string {
	var out []string
	for _, e := range f.entries {
		if e.Entity == entity && e.EntityID == entityID {
			out = append(out, e.Action)
		}
	}
	return out
}

func newAudit() (*AuditService, *fakeAuditRepo) {
	repo := &fakeAuditRepo{}
	return NewAuditService(repo), repo
}

// --- patient ---

type fakePatientRepo struct {
	byID    map[string]*patient.Patient
	byEmail map[string]*patient.Patient
	creates int
}

func newPatientRepo() *fakePatientRepo {
	return &fakePatientRepo{byID: map[string]*patient.Patient{}, byEmail: map[string]*patient.Patient{}}
}

func (f *fakePatientRepo) Create(_ context.Context, p *patient.Patient) error {
	f.creates++
	cp := *p
	f.byID[p.ID] = &cp
	f.byEmail[p.Email] = &cp
	return nil
}

func (f *fakePatientRepo) Update(_ context.Context, p *patient.Patient) error {
	cp := *p
	f.byID[p.ID] = &cp
	f.byEmail[p.Email] = &cp
	return nil
}

func (f *fakePatientRepo) Get(_ context.Context, id string) (*patient.Patient, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakePatientRepo) GetByEmail(_ context.Context, email string) (*patient.Patient, error) {
	p, ok := f.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakePatientRepo) List(_ context.Context) ([]*patient.Patient, error) {
	out := make([]*patient.Patient, 0, len(f.byID))
	for _, p := range f.byID {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakePatientRepo) ListByApproval(_ context.Context, status string) ([]*patient.Patient, error) {
	var out []*patient.Patient
	for _, p := range f.byID {
		if p.ApprovalStatus == status {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakePatientRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

func (f *fakePatientRepo) EmailExists(_ context.Context, email, exceptID string) (bool, error) {
	p, ok := f.byEmail[email]
	return ok && p.ID != exceptID, nil
}

func (f *fakePatientRepo) CPFExists(_ context.Context, cpf, exceptID string) (bool, error) {
	for _, p := range f.byID {
		if p.CPF == cpf && p.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}

// --- session ---

type fakeSessionRepo struct {
	byID    map[string]*scheduling.Session
	creates int
	updates int
}

func newSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{byID: map[string]*scheduling.Session{}}
}

func (f *fakeSessionRepo) Create(_ context.Context, s *scheduling.Session) error {
	f.creates++
	cp := *s
	f.byID[s.ID] = &cp
	return nil
}

func (f *fakeSessionRepo) Update(_ context.Context, s *scheduling.Session) error {
	f.updates++
	cp := *s
	f.byID[s.ID] = &cp
	return nil
}

func (f *fakeSessionRepo) Get(_ context.Context, id string) (*scheduling.Session, error) {
	s, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSessionRepo) List(_ context.Context) ([]*scheduling.Session, error) {
	out := make([]*scheduling.Session, 0, len(f.byID))
	for _, s := range f.byID {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeSessionRepo) ListByPatient(_ context.Context, patientID string) ([]*scheduling.Session, error) {
	var out []*scheduling.Session
	for _, s := range f.byID {
		if s.PatientID == patientID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepo) ListByRange(_ context.Context, from, to time.Time) ([]*scheduling.Session, error) {
	var out []*scheduling.Session
	for _, s := range f.byID {
		if inRange(s.StartsAt, from, to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepo) ListByLocationRange(_ context.Context, locationID string, from, to time.Time) ([]*scheduling.Session, error) {
	var out []*scheduling.Session
	for _, s := range f.byID {
		if s.LocationID == locationID && inRange(s.StartsAt, from, to) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}

// --- conflict/calendar ---

type fakeConflictChecker struct {
	conflict bool
	calls    int
}

func (f *fakeConflictChecker) HasConflict(_ context.Context, _, _ time.Time) (bool, error) {
	f.calls++
	return f.conflict, nil
}

type fakeCalendar struct {
	calls  int
	result CalendarEventResult
	last   CalendarEvent
}

func (f *fakeCalendar) CreateEvent(_ context.Context, ev CalendarEvent) (*CalendarEventResult, error) {
	f.calls++
	f.last = ev
	r := f.result
	return &r, nil
}

// recordingHook captura as sessões finalizadas.
type recordingHook struct {
	finished []*scheduling.Session
}

func (h *recordingHook) OnSessionFinished(_ context.Context, s *scheduling.Session) error {
	h.finished = append(h.finished, s)
	return nil
}

// --- plan ---

type fakePlanRepo struct {
	active     *billing.Plan // devolvido por GetActiveByPatient; nil ⇒ ErrNotFound
	fixedCycle []*billing.Plan
}

func (f *fakePlanRepo) Create(context.Context, *billing.Plan) error { return nil }
func (f *fakePlanRepo) Update(context.Context, *billing.Plan) error { return nil }
func (f *fakePlanRepo) Get(context.Context, string) (*billing.Plan, error) {
	return nil, ErrNotFound
}

func (f *fakePlanRepo) GetActiveByPatient(_ context.Context, _ string, _ time.Time) (*billing.Plan, error) {
	if f.active == nil {
		return nil, ErrNotFound
	}
	return f.active, nil
}

func (f *fakePlanRepo) ListByPatient(context.Context, string) ([]*billing.Plan, error) {
	return nil, nil
}

func (f *fakePlanRepo) ListFixedCycleActive(_ context.Context, _ time.Time) ([]*billing.Plan, error) {
	return f.fixedCycle, nil
}

func (f *fakePlanRepo) SoftDelete(context.Context, string) error { return nil }

// --- debt ---

type fakeDebtRepo struct {
	byID     map[string]*billing.Debt
	byKey    map[string]*billing.Debt
	inserts  int
	patients map[string][]*billing.Debt
}

func newDebtRepo() *fakeDebtRepo {
	return &fakeDebtRepo{byID: map[string]*billing.Debt{}, byKey: map[string]*billing.Debt{}, patients: map[string][]*billing.Debt{}}
}

func (f *fakeDebtRepo) InsertIfAbsent(_ context.Context, d *billing.Debt) (bool, error) {
	if _, ok := f.byKey[d.IdempotencyKey]; ok {
		return false, nil
	}
	f.inserts++
	cp := *d
	f.byKey[d.IdempotencyKey] = &cp
	f.byID[d.ID] = &cp
	f.patients[d.PatientID] = append(f.patients[d.PatientID], &cp)
	return true, nil
}

func (f *fakeDebtRepo) Get(_ context.Context, id string) (*billing.Debt, error) {
	d, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

func (f *fakeDebtRepo) GetByIdempotencyKey(_ context.Context, key string) (*billing.Debt, error) {
	d, ok := f.byKey[key]
	if !ok {
		return nil, ErrNotFound
	}
	return d, nil
}

func (f *fakeDebtRepo) List(_ context.Context) ([]*billing.Debt, error) {
	out := make([]*billing.Debt, 0, len(f.byID))
	for _, d := range f.byID {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeDebtRepo) ListByPatient(_ context.Context, patientID string) ([]*billing.Debt, error) {
	return f.patients[patientID], nil
}

func (f *fakeDebtRepo) ListBySessionIDs(_ context.Context, ids []string) ([]*billing.Debt, error) {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	var out []*billing.Debt
	for _, d := range f.byID {
		if d.SessionID != "" && set[d.SessionID] {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDebtRepo) ListByCreatedRange(_ context.Context, from, to time.Time) ([]*billing.Debt, error) {
	var out []*billing.Debt
	for _, d := range f.byID {
		if inRange(d.CreatedAt, from, to) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDebtRepo) ListDueBetween(_ context.Context, from, to time.Time) ([]*billing.Debt, error) {
	var out []*billing.Debt
	for _, d := range f.byID {
		if (d.Status == billing.DebtAberto || d.Status == billing.DebtParcial) && inRange(d.DueDate, from, to) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDebtRepo) UpdateStatus(_ context.Context, id, status string) error {
	if d, ok := f.byID[id]; ok {
		d.Status = status
	}
	return nil
}

func (f *fakeDebtRepo) SetPDF(_ context.Context, id, pdfFileID string) error {
	if d, ok := f.byID[id]; ok {
		d.PDFFileID = pdfFileID
	}
	return nil
}

// --- payment ---

type fakePaymentRepo struct {
	byDebt map[string][]*billing.Payment
}

func newPaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{byDebt: map[string][]*billing.Payment{}}
}

func (f *fakePaymentRepo) Insert(_ context.Context, p *billing.Payment) error {
	f.byDebt[p.DebtID] = append(f.byDebt[p.DebtID], p)
	return nil
}

func (f *fakePaymentRepo) ListByDebt(_ context.Context, debtID string) ([]*billing.Payment, error) {
	return f.byDebt[debtID], nil
}

func (f *fakePaymentRepo) SumByDebt(_ context.Context, debtID string) (int64, error) {
	var sum int64
	for _, p := range f.byDebt[debtID] {
		sum += p.Amount
	}
	return sum, nil
}

func (f *fakePaymentRepo) ListByRange(_ context.Context, from, to time.Time) ([]*billing.Payment, error) {
	var out []*billing.Payment
	for _, ps := range f.byDebt {
		for _, p := range ps {
			if inRange(p.PaidAt, from, to) {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// --- location ---

type fakeLocationRepo struct {
	byID map[string]*scheduling.Location
}

func newLocationRepo() *fakeLocationRepo {
	return &fakeLocationRepo{byID: map[string]*scheduling.Location{}}
}

func (f *fakeLocationRepo) Create(_ context.Context, l *scheduling.Location) error {
	f.byID[l.ID] = l
	return nil
}
func (f *fakeLocationRepo) Update(_ context.Context, l *scheduling.Location) error {
	f.byID[l.ID] = l
	return nil
}
func (f *fakeLocationRepo) Get(_ context.Context, id string) (*scheduling.Location, error) {
	l, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}
func (f *fakeLocationRepo) List(_ context.Context) ([]*scheduling.Location, error) {
	out := make([]*scheduling.Location, 0, len(f.byID))
	for _, l := range f.byID {
		out = append(out, l)
	}
	return out, nil
}
func (f *fakeLocationRepo) SoftDelete(_ context.Context, id string) error {
	delete(f.byID, id)
	return nil
}
func (f *fakeLocationRepo) AddAvailability(context.Context, *scheduling.Availability) error {
	return nil
}
func (f *fakeLocationRepo) ListAvailability(context.Context, string) ([]*scheduling.Availability, error) {
	return nil, nil
}
func (f *fakeLocationRepo) DeleteAvailability(context.Context, string) error { return nil }

// --- cost ---

type fakeCostRepo struct {
	sessionCosts map[string]*costs.SessionCost
	upserts      int
}

func newCostRepo() *fakeCostRepo {
	return &fakeCostRepo{sessionCosts: map[string]*costs.SessionCost{}}
}

func (f *fakeCostRepo) CreateCategory(context.Context, *costs.CostCategory) error { return nil }
func (f *fakeCostRepo) ListCategories(context.Context) ([]*costs.CostCategory, error) {
	return nil, nil
}
func (f *fakeCostRepo) CreateItem(context.Context, *costs.CostItem) error { return nil }
func (f *fakeCostRepo) ListItems(context.Context) ([]*costs.CostItem, error) {
	return nil, nil
}
func (f *fakeCostRepo) ListItemsByOrigin(context.Context, string) ([]*costs.CostItem, error) {
	return nil, nil
}
func (f *fakeCostRepo) SoftDeleteItem(context.Context, string) error { return nil }
func (f *fakeCostRepo) UpsertSessionCost(_ context.Context, sc *costs.SessionCost) error {
	f.upserts++
	f.sessionCosts[sc.SessionID] = sc
	return nil
}
func (f *fakeCostRepo) GetSessionCost(_ context.Context, sessionID string) (*costs.SessionCost, error) {
	sc, ok := f.sessionCosts[sessionID]
	if !ok {
		return nil, ErrNotFound
	}
	return sc, nil
}

// --- cost session source ---

type fakeCostSessionSource struct {
	count int
}

func (f *fakeCostSessionSource) CountRealizadasNoLocalNoPeriodo(context.Context, string, time.Time, time.Time) (int, error) {
	return f.count, nil
}

// --- prontuario / ged (para history/dayfeed) ---

type fakeProntuarioRepo struct {
	notes map[string][]*clinical.Note
	anam  map[string]*clinical.Anamnesis
}

func newProntuarioRepo() *fakeProntuarioRepo {
	return &fakeProntuarioRepo{notes: map[string][]*clinical.Note{}, anam: map[string]*clinical.Anamnesis{}}
}

func (f *fakeProntuarioRepo) GetAnamnesis(_ context.Context, patientID string) (*clinical.Anamnesis, error) {
	a, ok := f.anam[patientID]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}
func (f *fakeProntuarioRepo) UpsertAnamnesis(_ context.Context, a *clinical.Anamnesis) error {
	f.anam[a.PatientID] = a
	return nil
}
func (f *fakeProntuarioRepo) CreateNote(_ context.Context, n *clinical.Note) error {
	f.notes[n.PatientID] = append(f.notes[n.PatientID], n)
	return nil
}
func (f *fakeProntuarioRepo) ListNotes(_ context.Context, patientID string) ([]*clinical.Note, error) {
	return f.notes[patientID], nil
}
func (f *fakeProntuarioRepo) DeleteNote(context.Context, string) error { return nil }

type fakeGEDMetaRepo struct {
	byPatient map[string][]*domain.GEDFile
	all       []*domain.GEDFile
}

func newGEDMetaRepo() *fakeGEDMetaRepo {
	return &fakeGEDMetaRepo{byPatient: map[string][]*domain.GEDFile{}}
}

func (f *fakeGEDMetaRepo) Insert(_ context.Context, file *domain.GEDFile) error {
	f.byPatient[file.PatientID] = append(f.byPatient[file.PatientID], file)
	f.all = append(f.all, file)
	return nil
}
func (f *fakeGEDMetaRepo) Get(_ context.Context, id string) (*domain.GEDFile, error) {
	for _, file := range f.all {
		if file.ID == id {
			return file, nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeGEDMetaRepo) FindByHash(_ context.Context, patientID, sha string) (*domain.GEDFile, error) {
	for _, file := range f.all {
		if file.PatientID == patientID && file.SHA256 == sha {
			return file, nil
		}
	}
	return nil, ErrNotFound
}

// fakeGEDBlob é um GEDBlobStore em memória (mapa relPath -> bytes).
type fakeGEDBlob struct {
	mu    sync.Mutex
	store map[string][]byte
	puts  int
}

func newGEDBlob() *fakeGEDBlob { return &fakeGEDBlob{store: map[string][]byte{}} }

func (f *fakeGEDBlob) Put(_ context.Context, relPath, _ string, content []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	cp := make([]byte, len(content))
	copy(cp, content)
	f.store[relPath] = cp
	return nil
}

func (f *fakeGEDBlob) Get(_ context.Context, relPath string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.store[relPath]
	if !ok {
		return nil, ErrNotFound
	}
	return b, nil
}
func (f *fakeGEDMetaRepo) ListByPatient(_ context.Context, patientID string) ([]*domain.GEDFile, error) {
	return f.byPatient[patientID], nil
}
func (f *fakeGEDMetaRepo) ListByCreatedRange(_ context.Context, from, to time.Time) ([]*domain.GEDFile, error) {
	var out []*domain.GEDFile
	for _, file := range f.all {
		if inRange(file.CreatedAt, from, to) {
			out = append(out, file)
		}
	}
	return out, nil
}
func (f *fakeGEDMetaRepo) Delete(context.Context, string) error { return nil }

// --- appointment ---

type fakeAppointmentRepo struct {
	pending []*scheduling.AppointmentRequest
}

func (f *fakeAppointmentRepo) Create(context.Context, *scheduling.AppointmentRequest) error {
	return nil
}
func (f *fakeAppointmentRepo) Get(context.Context, string) (*scheduling.AppointmentRequest, error) {
	return nil, ErrNotFound
}
func (f *fakeAppointmentRepo) ListPending(context.Context) ([]*scheduling.AppointmentRequest, error) {
	return f.pending, nil
}
func (f *fakeAppointmentRepo) ListByPatient(context.Context, string) ([]*scheduling.AppointmentRequest, error) {
	return nil, nil
}
func (f *fakeAppointmentRepo) UpdateStatus(context.Context, string, string) error { return nil }

// --- IA (Gemini) ---

// fakeSummarizer registra os inputs (asserção de minimização) e devolve um
// resultado determinístico.
type fakeSummarizer struct {
	result AISummaryOutput
	err    error
	calls  []AISummaryInput
}

func (f *fakeSummarizer) Summarize(_ context.Context, in AISummaryInput) (*AISummaryOutput, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	r := f.result
	return &r, nil
}

// fakeAISummaryRepo guarda os resumos em memória.
type fakeAISummaryRepo struct {
	byID map[string]*clinical.AISummary
}

func newAISummaryRepo() *fakeAISummaryRepo {
	return &fakeAISummaryRepo{byID: map[string]*clinical.AISummary{}}
}

func (f *fakeAISummaryRepo) Create(_ context.Context, s *clinical.AISummary) error {
	cp := *s
	f.byID[s.ID] = &cp
	return nil
}
func (f *fakeAISummaryRepo) Get(_ context.Context, id string) (*clinical.AISummary, error) {
	s, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}
func (f *fakeAISummaryRepo) GetBySession(_ context.Context, sessionID string) (*clinical.AISummary, error) {
	for _, s := range f.byID {
		if s.SessionID == sessionID {
			return s, nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeAISummaryRepo) UpdateStatus(_ context.Context, id, status string) error {
	if s, ok := f.byID[id]; ok {
		s.Status = status
		return nil
	}
	return ErrNotFound
}

// fakeTagIndexRepo é o índice invertido em memória, chaveado por
// (patient_id, ano, tag) -> conjunto de session_ids.
type fakeTagIndexRepo struct {
	entries map[string][]string // "pid|ano|tag" -> session_ids
}

func newTagIndexRepo() *fakeTagIndexRepo {
	return &fakeTagIndexRepo{entries: map[string][]string{}}
}

func tagKey(pid string, ano int, tag string) string {
	return pid + "|" + strconv.Itoa(ano) + "|" + tag
}

func (f *fakeTagIndexRepo) Upsert(_ context.Context, patientID string, ano int, tag, sessionID string) error {
	k := tagKey(patientID, ano, tag)
	for _, id := range f.entries[k] {
		if id == sessionID {
			return nil
		}
	}
	f.entries[k] = append(f.entries[k], sessionID)
	return nil
}
func (f *fakeTagIndexRepo) FindByTag(_ context.Context, patientID string, ano int, tag string) ([]string, error) {
	ids, ok := f.entries[tagKey(patientID, ano, tag)]
	if !ok {
		return nil, ErrNotFound
	}
	return ids, nil
}
func (f *fakeTagIndexRepo) ListTags(_ context.Context, patientID string, ano int) ([]*clinical.TagIndex, error) {
	var out []*clinical.TagIndex
	prefix := patientID + "|" + strconv.Itoa(ano) + "|"
	for k, ids := range f.entries {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, &clinical.TagIndex{PatientID: patientID, Ano: ano, Tag: k[len(prefix):], SessionIDs: ids})
		}
	}
	return out, nil
}
