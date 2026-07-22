package domain

import "time"

type Role string

const (
	RoleAdmin        Role = "admin"
	RolePsychologist Role = "psychologist"
)

type AppointmentType string

const (
	AppointmentInPerson AppointmentType = "in_person"
	AppointmentOnline   AppointmentType = "online"
)

type AppointmentStatus string

const (
	StatusScheduled   AppointmentStatus = "scheduled"
	StatusCancelled   AppointmentStatus = "cancelled"
	StatusCompleted   AppointmentStatus = "completed"
	StatusRescheduled AppointmentStatus = "rescheduled"
)

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentReceived PaymentStatus = "received"
)

type DocType string

const (
	DocLaudo      DocType = "laudo"
	DocNotaFiscal DocType = "nota_fiscal"
	DocRelatorio  DocType = "relatorio"
	DocOutro      DocType = "outro"
)

type UploadedBy string

const (
	UploadedByPsychologist UploadedBy = "psychologist"
	UploadedByPatient      UploadedBy = "patient"
	UploadedBySystem       UploadedBy = "system"
)

type StaffUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Patient struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	Phone     string     `json:"phone"`
	BirthDate *time.Time `json:"birth_date"`
	GoogleSub string     `json:"google_sub,omitempty"`
	Anamnesis string     `json:"anamnesis"`
	CreatedAt time.Time  `json:"created_at"`
}

type Appointment struct {
	ID                 string            `json:"id"`
	PatientID          string            `json:"patient_id"`
	PatientName        string            `json:"patient_name"`
	Type               AppointmentType   `json:"type"`
	Status             AppointmentStatus `json:"status"`
	ScheduledAt        time.Time         `json:"scheduled_at"`
	DurationMinutes    int               `json:"duration_minutes"`
	GoogleEventID      string            `json:"google_event_id,omitempty"`
	MeetLink           string            `json:"meet_link"`
	Notes              string            `json:"notes"`
	ReportHTML         string            `json:"report_html"`
	CancellationReason string            `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type SchedulingRules struct {
	MinHoursToCancel       int  `json:"min_hours_to_cancel"`
	MinHoursToReschedule   int  `json:"min_hours_to_reschedule"`
	MaxReschedulesPerMonth int  `json:"max_reschedules_per_month"`
	AllowPatientCancel     bool `json:"allow_patient_cancel"`
	AllowPatientReschedule bool `json:"allow_patient_reschedule"`
}

type Payment struct {
	ID            string        `json:"id"`
	PatientID     string        `json:"patient_id"`
	PatientName   string        `json:"patient_name"`
	AppointmentID string        `json:"appointment_id,omitempty"`
	AmountCents   int64         `json:"amount_cents"`
	Status        PaymentStatus `json:"status"`
	DueDate       time.Time     `json:"due_date"`
	ReceivedAt    *time.Time    `json:"received_at"`
	InvoiceNumber string        `json:"invoice_number,omitempty"`
}

type Cost struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
	Month       int    `json:"month"`
	Year        int    `json:"year"`
	Category    string `json:"category"`
}

type Document struct {
	ID            string     `json:"id"`
	PatientID     string     `json:"patient_id"`
	AppointmentID string     `json:"appointment_id,omitempty"`
	Filename      string     `json:"filename"`
	MimeType      string     `json:"mime_type"`
	Path          string     `json:"path"`
	UploadedBy    UploadedBy `json:"uploaded_by"`
	DocType       DocType    `json:"doc_type"`
	CreatedAt     time.Time  `json:"created_at"`
}

type MonthlyReport struct {
	Month        int           `json:"month"`
	Year         int           `json:"year"`
	PatientID    string        `json:"patient_id"`
	PatientName  string        `json:"patient_name"`
	Appointments []Appointment `json:"appointments"`
	TotalAmount  int64         `json:"total_amount"`
}

type FinanceSummary struct {
	Month         int       `json:"month"`
	Year          int       `json:"year"`
	TotalReceived int64     `json:"total_received"`
	TotalPending  int64     `json:"total_pending"`
	TotalCosts    int64     `json:"total_costs"`
	Balance       int64     `json:"balance"`
	Payments      []Payment `json:"payments"`
	Costs         []Cost    `json:"costs"`
}

type AvailableSlot struct {
	Start           time.Time `json:"start"`
	DurationMinutes int       `json:"duration_minutes"`
}

// ─── Session Notes ───────────────────────────────────────────────────────────

type SessionNote struct {
	ID                  string    `json:"id"`
	AppointmentID       string    `json:"appointment_id"`
	PatientID           string    `json:"patient_id"`
	PatientName         string    `json:"patient_name,omitempty"`
	ContentHTML         string    `json:"content_html"`
	PrivateNotes        string    `json:"private_notes"`
	DurationPatientMin  int       `json:"duration_patient_min"`
	DurationAnalysisMin int       `json:"duration_analysis_min"`
	DurationAdminMin    int       `json:"duration_admin_min"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ─── Anamnesis Templates ─────────────────────────────────────────────────────

type AgeGroup string

const (
	AgeGroupAdult     AgeGroup = "adult"
	AgeGroupChild     AgeGroup = "child"
	AgeGroupUniversal AgeGroup = "universal"
)

type AnamnesisFieldType string

const (
	FieldText     AnamnesisFieldType = "text"
	FieldTextarea AnamnesisFieldType = "textarea"
	FieldSelect   AnamnesisFieldType = "select"
	FieldCheckbox AnamnesisFieldType = "checkbox"
	FieldDate     AnamnesisFieldType = "date"
	FieldNumber   AnamnesisFieldType = "number"
	FieldScale    AnamnesisFieldType = "scale"
)

type AnamnesisField struct {
	Key      string             `json:"key"`
	Label    string             `json:"label"`
	Type     AnamnesisFieldType `json:"type"`
	Required bool               `json:"required"`
	Options  []string           `json:"options,omitempty"`
}

type AnamnesisTemplate struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	TargetAgeGroup AgeGroup         `json:"target_age_group"`
	Fields         []AnamnesisField `json:"fields"`
	IsActive       bool             `json:"is_active"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type AnamnesisResponse struct {
	ID           string            `json:"id"`
	PatientID    string            `json:"patient_id"`
	PatientName  string            `json:"patient_name,omitempty"`
	TemplateID   string            `json:"template_id"`
	TemplateName string            `json:"template_name,omitempty"`
	Responses    map[string]string `json:"responses"`
	CompletedAt  *time.Time        `json:"completed_at"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ─── Therapeutic Contract ────────────────────────────────────────────────────

type ContractStatus string

const (
	ContractPending ContractStatus = "pending"
	ContractSigned  ContractStatus = "signed"
	ContractExpired ContractStatus = "expired"
	ContractRevoked ContractStatus = "revoked"
)

type ContractTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentHTML string    `json:"content_html"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type Contract struct {
	ID                 string         `json:"id"`
	PatientID          string         `json:"patient_id"`
	PatientName        string         `json:"patient_name,omitempty"`
	TemplateID         string         `json:"template_id"`
	TemplateName       string         `json:"template_name,omitempty"`
	Status             ContractStatus `json:"status"`
	GeneratedHTML      string         `json:"generated_html"`
	SignedAt           *time.Time     `json:"signed_at"`
	SignatureIP        string         `json:"signature_ip,omitempty"`
	SignatureUserAgent string         `json:"signature_user_agent,omitempty"`
	PDFPath            string         `json:"pdf_path,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

// ─── Supervisões ─────────────────────────────────────────────────────────────

type Supervisor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Specialty string    `json:"specialty"`
	CRP       string    `json:"crp"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

type SupervisionStatus string

const (
	SupervisionScheduled SupervisionStatus = "scheduled"
	SupervisionCompleted SupervisionStatus = "completed"
	SupervisionCancelled SupervisionStatus = "cancelled"
)

type SupervisionSession struct {
	ID              string            `json:"id"`
	SupervisorID    string            `json:"supervisor_id"`
	SupervisorName  string            `json:"supervisor_name,omitempty"`
	ScheduledAt     time.Time         `json:"scheduled_at"`
	DurationMinutes int               `json:"duration_minutes"`
	NotesHTML       string            `json:"notes_html"`
	Topics          string            `json:"topics"`
	CostCents       int64             `json:"cost_cents"`
	Status          SupervisionStatus `json:"status"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ─── Espaços / Consultórios ──────────────────────────────────────────────────

type SpaceType string

const (
	SpaceFixed     SpaceType = "fixed"
	SpaceRented    SpaceType = "rented"
	SpaceTemporary SpaceType = "temporary"
)

type TherapySpace struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Address          string    `json:"address"`
	Type             SpaceType `json:"type"`
	CostCentsPerUse  int64     `json:"cost_cents_per_use"`
	CostCentsMonthly int64     `json:"cost_cents_monthly"`
	IsAvailable      bool      `json:"is_available"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
}

type SpaceBooking struct {
	ID            string    `json:"id"`
	SpaceID       string    `json:"space_id"`
	SpaceName     string    `json:"space_name,omitempty"`
	AppointmentID string    `json:"appointment_id,omitempty"`
	BookingDate   string    `json:"booking_date"`
	StartTime     string    `json:"start_time"`
	EndTime       string    `json:"end_time"`
	CreatedAt     time.Time `json:"created_at"`
}
