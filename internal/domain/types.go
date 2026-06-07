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
	ID        string
	Email     string
	Role      Role
	CreatedAt time.Time
}

type Patient struct {
	ID        string
	Email     string
	Name      string
	Phone     string
	BirthDate *time.Time
	GoogleSub string
	Anamnesis string
	CreatedAt time.Time
}

type Appointment struct {
	ID                 string
	PatientID          string
	PatientName        string
	Type               AppointmentType
	Status             AppointmentStatus
	ScheduledAt        time.Time
	DurationMinutes    int
	GoogleEventID      string
	MeetLink           string
	Notes              string
	ReportHTML         string
	CancellationReason string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type SchedulingRules struct {
	MinHoursToCancel       int
	MinHoursToReschedule   int
	MaxReschedulesPerMonth int
	AllowPatientCancel     bool
	AllowPatientReschedule bool
}

type Payment struct {
	ID            string
	PatientID     string
	PatientName   string
	AppointmentID string
	AmountCents   int64
	Status        PaymentStatus
	DueDate       time.Time
	ReceivedAt    *time.Time
	InvoiceNumber string
}

type Cost struct {
	ID          string
	Description string
	AmountCents int64
	Month       int
	Year        int
	Category    string
}

type Document struct {
	ID            string
	PatientID     string
	AppointmentID string
	Filename      string
	MimeType      string
	Path          string
	UploadedBy    UploadedBy
	DocType       DocType
	CreatedAt     time.Time
}

type MonthlyReport struct {
	Month        int
	Year         int
	PatientID    string
	PatientName  string
	Appointments []Appointment
	TotalAmount  int64
}

type FinanceSummary struct {
	Month         int
	Year          int
	TotalReceived int64
	TotalPending  int64
	TotalCosts    int64
	Balance       int64
	Payments      []Payment
	Costs         []Cost
}

type AvailableSlot struct {
	Start           time.Time
	DurationMinutes int
}
