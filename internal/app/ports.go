package app

import (
	"context"
	"time"

	"github.com/RafaelFino/psicoman/internal/domain"
	"github.com/RafaelFino/psicoman/internal/domain/billing"
	"github.com/RafaelFino/psicoman/internal/domain/clinical"
	"github.com/RafaelFino/psicoman/internal/domain/costs"
	"github.com/RafaelFino/psicoman/internal/domain/patient"
	"github.com/RafaelFino/psicoman/internal/domain/scheduling"
)

// Portas de persistência por bounded context (decisão 5 do fase2-plan.md): as
// interfaces de repositório vivem em internal/app, junto dos serviços que as
// consomem, tipadas contra os pacotes de domínio. O adapter platform/pb (Fase 3)
// as implementa. Toda função de porta recebe context.Context como 1º parâmetro.

// --- patient ---

// PatientRepository persiste pacientes.
type PatientRepository interface {
	Create(ctx context.Context, p *patient.Patient) error
	Update(ctx context.Context, p *patient.Patient) error
	Get(ctx context.Context, id string) (*patient.Patient, error)
	GetByEmail(ctx context.Context, email string) (*patient.Patient, error)
	List(ctx context.Context) ([]*patient.Patient, error)
	ListByApproval(ctx context.Context, status string) ([]*patient.Patient, error)
	SoftDelete(ctx context.Context, id string) error
	// EmailExists / CPFExists checam unicidade ignorando um id (para update).
	EmailExists(ctx context.Context, email, exceptID string) (bool, error)
	CPFExists(ctx context.Context, cpf, exceptID string) (bool, error)
}

// OriginRepository persiste canais de aquisição.
type OriginRepository interface {
	Create(ctx context.Context, o *patient.Origin) error
	Update(ctx context.Context, o *patient.Origin) error
	Get(ctx context.Context, id string) (*patient.Origin, error)
	List(ctx context.Context) ([]*patient.Origin, error)
	SoftDelete(ctx context.Context, id string) error
}

// --- scheduling ---

// SessionRepository persiste sessões.
type SessionRepository interface {
	Create(ctx context.Context, s *scheduling.Session) error
	Update(ctx context.Context, s *scheduling.Session) error
	Get(ctx context.Context, id string) (*scheduling.Session, error)
	List(ctx context.Context) ([]*scheduling.Session, error)
	ListByPatient(ctx context.Context, patientID string) ([]*scheduling.Session, error)
	// ListByRange devolve as sessões cujo início está em [from, to).
	ListByRange(ctx context.Context, from, to time.Time) ([]*scheduling.Session, error)
	// ListByLocationRange devolve as sessões de um local cujo início está em
	// [from, to). Limites zero = sem limite (histórico completo do local).
	ListByLocationRange(ctx context.Context, locationID string, from, to time.Time) ([]*scheduling.Session, error)
	SoftDelete(ctx context.Context, id string) error
}

// AppointmentRepository persiste pedidos de agendamento.
type AppointmentRepository interface {
	Create(ctx context.Context, a *scheduling.AppointmentRequest) error
	Get(ctx context.Context, id string) (*scheduling.AppointmentRequest, error)
	ListPending(ctx context.Context) ([]*scheduling.AppointmentRequest, error)
	ListByPatient(ctx context.Context, patientID string) ([]*scheduling.AppointmentRequest, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

// LocationRepository persiste locais e disponibilidades.
type LocationRepository interface {
	Create(ctx context.Context, l *scheduling.Location) error
	Update(ctx context.Context, l *scheduling.Location) error
	Get(ctx context.Context, id string) (*scheduling.Location, error)
	List(ctx context.Context) ([]*scheduling.Location, error)
	SoftDelete(ctx context.Context, id string) error
	AddAvailability(ctx context.Context, a *scheduling.Availability) error
	ListAvailability(ctx context.Context, locationID string) ([]*scheduling.Availability, error)
	DeleteAvailability(ctx context.Context, id string) error
}

// --- billing ---

// PlanRepository persiste planos de cobrança.
type PlanRepository interface {
	Create(ctx context.Context, p *billing.Plan) error
	Update(ctx context.Context, p *billing.Plan) error
	Get(ctx context.Context, id string) (*billing.Plan, error)
	// GetActiveByPatient devolve o plano vigente do paciente em ref (ErrNotFound se nenhum).
	GetActiveByPatient(ctx context.Context, patientID string, ref time.Time) (*billing.Plan, error)
	ListByPatient(ctx context.Context, patientID string) ([]*billing.Plan, error)
	// ListFixedCycleActive devolve planos fechados vigentes em ref.
	ListFixedCycleActive(ctx context.Context, ref time.Time) ([]*billing.Plan, error)
	SoftDelete(ctx context.Context, id string) error
}

// ContractRepository persiste contratos (entidade nova do MER, D4b.2).
type ContractRepository interface {
	Create(ctx context.Context, c *billing.Contract) error
	Update(ctx context.Context, c *billing.Contract) error
	Get(ctx context.Context, id string) (*billing.Contract, error)
	ListByPatient(ctx context.Context, patientID string) ([]*billing.Contract, error)
	SoftDelete(ctx context.Context, id string) error
}

// DebtRepository persiste débitos, com geração idempotente por chave.
type DebtRepository interface {
	// InsertIfAbsent grava o débito só se a idempotency_key ainda não existir.
	// Devolve (created=true) quando inseriu; false quando já existia.
	InsertIfAbsent(ctx context.Context, d *billing.Debt) (bool, error)
	Get(ctx context.Context, id string) (*billing.Debt, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*billing.Debt, error)
	List(ctx context.Context) ([]*billing.Debt, error)
	ListByPatient(ctx context.Context, patientID string) ([]*billing.Debt, error)
	// ListBySessionIDs devolve os débitos vinculados a qualquer uma das sessões.
	ListBySessionIDs(ctx context.Context, sessionIDs []string) ([]*billing.Debt, error)
	// ListByCreatedRange devolve os débitos criados em [from, to).
	ListByCreatedRange(ctx context.Context, from, to time.Time) ([]*billing.Debt, error)
	// ListDueBetween devolve os débitos em aberto/parcial com vencimento em [from, to).
	ListDueBetween(ctx context.Context, from, to time.Time) ([]*billing.Debt, error)
	UpdateStatus(ctx context.Context, id, status string) error
	SetPDF(ctx context.Context, id, pdfFileID string) error
}

// PaymentRepository persiste pagamentos.
type PaymentRepository interface {
	Insert(ctx context.Context, p *billing.Payment) error
	ListByDebt(ctx context.Context, debtID string) ([]*billing.Payment, error)
	// SumByDebt devolve o total já pago de um débito.
	SumByDebt(ctx context.Context, debtID string) (int64, error)
	// ListByRange devolve os pagamentos recebidos (paid_at) em [from, to).
	ListByRange(ctx context.Context, from, to time.Time) ([]*billing.Payment, error)
}

// --- costs ---

// CostRepository persiste categorias, itens e custos de sessão.
type CostRepository interface {
	CreateCategory(ctx context.Context, c *costs.CostCategory) error
	ListCategories(ctx context.Context) ([]*costs.CostCategory, error)
	CreateItem(ctx context.Context, i *costs.CostItem) error
	ListItems(ctx context.Context) ([]*costs.CostItem, error)
	ListItemsByOrigin(ctx context.Context, originID string) ([]*costs.CostItem, error)
	SoftDeleteItem(ctx context.Context, id string) error
	UpsertSessionCost(ctx context.Context, sc *costs.SessionCost) error
	GetSessionCost(ctx context.Context, sessionID string) (*costs.SessionCost, error)
}

// CostSessionSource fornece as sessões realizadas de um local num período, para
// o rateio proporcional (implementado pelo SessionRepository via adapter).
type CostSessionSource interface {
	// CountRealizadasNoLocalNoPeriodo conta sessões realizadas no local entre
	// [de, ate), considerando apenas as que computam custo.
	CountRealizadasNoLocalNoPeriodo(ctx context.Context, locationID string, de, ate time.Time) (int, error)
}

// ReportRepository fornece agregações para relatórios financeiros, de custos e ROI.
type ReportRepository interface {
	// RevenueByOrigin soma o valor recebido (pagamentos) por origem no período.
	RevenueByOrigin(ctx context.Context, de, ate time.Time) (map[string]int64, error)
	// DebtsSummary agrega débitos gerados, em aberto e recebidos no período.
	DebtsSummary(ctx context.Context, de, ate time.Time) (*costs.FinancialSummary, error)
	// CostTotalByKind soma os custos periodizados no intervalo (proporcional ao período).
	CostTotalByKind(ctx context.Context, de, ate time.Time) (map[string]int64, error)
	// SessionCostByPatient soma o custo atribuído às sessões de cada paciente.
	SessionCostByPatient(ctx context.Context, de, ate time.Time) (map[string]int64, error)
}

// --- clinical ---

// ProntuarioRepository persiste anamnese e notas de prontuário.
type ProntuarioRepository interface {
	GetAnamnesis(ctx context.Context, patientID string) (*clinical.Anamnesis, error)
	UpsertAnamnesis(ctx context.Context, a *clinical.Anamnesis) error
	CreateNote(ctx context.Context, n *clinical.Note) error
	ListNotes(ctx context.Context, patientID string) ([]*clinical.Note, error)
	DeleteNote(ctx context.Context, id string) error
}

// AISummaryRepository persiste os resumos de IA (resumo clínico cifrado na
// borda). O ciclo de revisão (sugerido → revisado → consolidado) é gravado no
// campo status.
type AISummaryRepository interface {
	Create(ctx context.Context, s *clinical.AISummary) error
	Get(ctx context.Context, id string) (*clinical.AISummary, error)
	GetBySession(ctx context.Context, sessionID string) (*clinical.AISummary, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

// TagIndexRepository persiste o índice invertido de tags por paciente/ano (tags
// em CLARO). Isolado por paciente: nenhuma consulta cruza pacientes (R10.2/R6).
type TagIndexRepository interface {
	// Upsert adiciona o sessionID à entrada (patient_id, ano, tag), criando-a
	// se não existir (merge idempotente — não duplica sessão numa tag).
	Upsert(ctx context.Context, patientID string, ano int, tag, sessionID string) error
	// FindByTag devolve as sessões que carregam a tag no ano, para o paciente.
	FindByTag(ctx context.Context, patientID string, ano int, tag string) ([]string, error)
	// ListTags devolve as tags do paciente no ano (para navegação).
	ListTags(ctx context.Context, patientID string, ano int) ([]*clinical.TagIndex, error)
}

// --- transversais ---

// GEDMetaRepository persiste os metadados dos arquivos do GED.
type GEDMetaRepository interface {
	Insert(ctx context.Context, f *domain.GEDFile) error
	Get(ctx context.Context, id string) (*domain.GEDFile, error)
	// FindByHash busca dedup dentro do escopo do paciente ("" = perfil terapeuta).
	FindByHash(ctx context.Context, patientID, sha string) (*domain.GEDFile, error)
	ListByPatient(ctx context.Context, patientID string) ([]*domain.GEDFile, error)
	// ListByCreatedRange devolve os arquivos criados em [from, to) (todos os pacientes).
	ListByCreatedRange(ctx context.Context, from, to time.Time) ([]*domain.GEDFile, error)
	Delete(ctx context.Context, id string) error
}

// AuditRepository persiste registros de auditoria.
type AuditRepository interface {
	Insert(ctx context.Context, entry *domain.AuditLog) error
	List(ctx context.Context, limit int) ([]*domain.AuditLog, error)
	// ListByEntity devolve as entradas de uma entidade específica (ex.: todas as
	// de uma sessão), mais recentes primeiro. Usado pela composição do histórico.
	ListByEntity(ctx context.Context, entity, entityID string) ([]*domain.AuditLog, error)
}

// --- integração consumida pelo SessionService ---
//
// Contratos mínimos consumidos pelo SessionService. A implementação real
// (freebusy/evento do Google Calendar) chega numa fase posterior; aqui só os
// contratos permitem orquestração + fakes nos testes.

// ConflictChecker verifica conflito de agenda antes de agendar uma sessão.
type ConflictChecker interface {
	HasConflict(ctx context.Context, startsAt, endsAt time.Time) (bool, error)
}

// CalendarEvent descreve o evento a criar no Google Calendar ao efetivar uma
// sessão (resumo/descrição/horários/convidado/lembretes/Meet).
type CalendarEvent struct {
	Summary         string
	Description     string
	StartsAt        time.Time
	EndsAt          time.Time
	AttendeeEmail   string
	ReminderMinutes []int
	WithMeet        bool
}

// CalendarEventResult é o resultado da criação do evento (ids/URL úteis).
type CalendarEventResult struct {
	EventID string
	MeetURL string
}

// SessionCalendar cria o evento no Google Calendar ao efetivar a sessão.
// Opcional: sem Google configurado, a sessão é agendada sem evento.
type SessionCalendar interface {
	CreateEvent(ctx context.Context, event CalendarEvent) (*CalendarEventResult, error)
}

// SessionFinishedHook é chamado ao finalizar uma sessão (status=realizada),
// permitindo que o financeiro/custos reajam.
type SessionFinishedHook interface {
	OnSessionFinished(ctx context.Context, s *scheduling.Session) error
}

// --- storage de binários do GED ---

// GEDBlobStore persiste e recupera os BYTES dos arquivos do GED. É a porta do
// file storage do PocketBase (adapter platform/pb sobre app.NewFilesystem(),
// local ou S3 — D9b): o metadado (dedup por sha256, relations) fica no
// GEDMetaRepository; aqui só o conteúdo, endereçado por relPath (fileKey).
// O serviço (InvoiceService) orquestra os dois: hash → dedup → Put + Insert.
type GEDBlobStore interface {
	// Put grava os bytes sob relPath (sobrescreve se já existir — idempotente
	// para o mesmo conteúdo, pois relPath deriva do sha256).
	Put(ctx context.Context, relPath, mime string, content []byte) error
	// Get devolve os bytes gravados sob relPath.
	Get(ctx context.Context, relPath string) ([]byte, error)
}
