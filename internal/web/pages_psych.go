package web

import (
	"net/http"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/gin-gonic/gin"
)

// registerPsychPages registers HTML page routes for the psych interface.
func (a *App) registerPsychPages(r *gin.RouterGroup) {
	r.GET("", a.pagePsychDashboard)
	r.GET("/patients", a.pagePsychPatients)
	r.GET("/patients/:id", a.pagePsychPatientDetail)
	r.GET("/appointments", a.pagePsychAppointments)
	r.GET("/session-notes", a.pagePsychSessionNotes)
	r.GET("/anamnesis", a.pagePsychAnamnesis)
	r.GET("/contracts", a.pagePsychContracts)
	r.GET("/supervisions", a.pagePsychSupervisions)
	r.GET("/spaces", a.pagePsychSpaces)
	r.GET("/finance", a.pagePsychFinance)
	r.GET("/settings", a.pagePsychSettings)
}

// ─── Dashboard (Agenda) ─────────────────────────────────────────────────────

type WeekDay struct {
	Date         time.Time
	DayName      string
	DayNumber    int
	IsToday      bool
	Appointments []domain.Appointment
}

type dashboardData struct {
	PageData
	TodayAppts    []domain.Appointment
	WeekAppts     []domain.Appointment
	UpcomingAppts []domain.Appointment
	WeekDays      []WeekDay
	PatientCount  int
	MonthHours    string
}

func (a *App) pagePsychDashboard(c *gin.Context) {
	db := getDB(c)
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayEnd := todayStart.AddDate(0, 0, 1)
	weekEnd := todayStart.AddDate(0, 0, 7)

	todayAppts, _ := db.ListAppointments(todayStart, todayEnd, "")
	weekAppts, _ := db.ListAppointments(todayStart, weekEnd, "")
	patients, _ := db.ListPatients()

	// Upcoming = next 7 days excluding today
	var upcoming []domain.Appointment
	for _, a := range weekAppts {
		if a.ScheduledAt.After(todayEnd) {
			upcoming = append(upcoming, a)
		}
	}

	// Build week days for weekly grid view
	dayNames := []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}
	weekDays := make([]WeekDay, 7)
	for i := 0; i < 7; i++ {
		day := todayStart.AddDate(0, 0, i)
		dayEnd2 := day.AddDate(0, 0, 1)
		wd := WeekDay{Date: day, DayName: dayNames[day.Weekday()], DayNumber: day.Day(), IsToday: i == 0}
		for _, appt := range weekAppts {
			if !appt.ScheduledAt.Before(day) && appt.ScheduledAt.Before(dayEnd2) {
				wd.Appointments = append(wd.Appointments, appt)
			}
		}
		weekDays[i] = wd
	}

	// Monthly hours
	patientMin, analysisMin, adminMin, _ := db.SessionNoteHoursForMonth(int(now.Month()), now.Year())
	totalHours := (patientMin + analysisMin + adminMin) / 60

	hoursStr := "0h"
	if totalHours > 0 {
		hoursStr = time.Duration(time.Duration(patientMin+analysisMin+adminMin) * time.Minute).String()
	}

	data := dashboardData{
		PageData:      basePsychData("dashboard"),
		TodayAppts:    todayAppts,
		WeekAppts:     weekAppts,
		UpcomingAppts: upcoming,
		WeekDays:      weekDays,
		PatientCount:  len(patients),
		MonthHours:    hoursStr,
	}
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/dashboard", data)
}

// ─── Patients ────────────────────────────────────────────────────────────────

type patientsData struct {
	PageData
	Patients []domain.Patient
}

func (a *App) pagePsychPatients(c *gin.Context) {
	db := getDB(c)
	patients, _ := db.ListPatients()
	if patients == nil {
		patients = []domain.Patient{}
	}
	data := patientsData{
		PageData: basePsychData("patients"),
		Patients: patients,
	}
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/patients", data)
}

// ─── Patient Detail (360°) ───────────────────────────────────────────────────

type patientDetailData struct {
	PageData
	Patient            *domain.Patient
	Appointments       []domain.Appointment
	SessionNotes       []domain.SessionNote
	Documents          []domain.Document
	Contracts          []domain.Contract
	Payments           []domain.Payment
	AnamnesisResponses []domain.AnamnesisResponse
}

func (a *App) pagePsychPatientDetail(c *gin.Context) {
	db := getDB(c)
	id := c.Param("id")

	patient, err := db.GetPatient(id)
	if err != nil {
		c.String(http.StatusNotFound, "Paciente não encontrado")
		return
	}

	// Load all related data
	from := patient.CreatedAt
	to := time.Now().UTC().AddDate(1, 0, 0)
	appts, _ := db.ListAppointments(from, to, id)
	notes, _ := db.ListSessionNotes(id)
	docs, _ := db.ListDocuments(id)
	contracts, _ := db.ListContracts(id)
	responses, _ := db.ListAnamnesisResponses(id)

	// Load payments for this patient (all months)
	// We'll get the current year's payments as an approximation
	now := time.Now().UTC()
	var payments []domain.Payment
	for m := 1; m <= 12; m++ {
		monthly, _ := db.ListPayments(m, now.Year())
		for _, p := range monthly {
			if p.PatientID == id {
				payments = append(payments, p)
			}
		}
	}

	data := patientDetailData{
		PageData:           basePsychData("patients"),
		Patient:            patient,
		Appointments:       appts,
		SessionNotes:       notes,
		Documents:          docs,
		Contracts:          contracts,
		Payments:           payments,
		AnamnesisResponses: responses,
	}
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/patient_detail", data)
}

// ─── Appointments ────────────────────────────────────────────────────────────

func (a *App) pagePsychAppointments(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/appointments", basePsychData("appointments"))
}

// ─── Session Notes ───────────────────────────────────────────────────────────

func (a *App) pagePsychSessionNotes(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/session_notes", basePsychData("session-notes"))
}

// ─── Anamnesis ───────────────────────────────────────────────────────────────

func (a *App) pagePsychAnamnesis(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/anamnesis", basePsychData("anamnesis"))
}

// ─── Contracts ───────────────────────────────────────────────────────────────

func (a *App) pagePsychContracts(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/contracts", basePsychData("contracts"))
}

// ─── Supervisions ────────────────────────────────────────────────────────────

func (a *App) pagePsychSupervisions(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/supervisions", basePsychData("supervisions"))
}

// ─── Spaces ──────────────────────────────────────────────────────────────────

func (a *App) pagePsychSpaces(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/spaces", basePsychData("spaces"))
}

// ─── Finance ─────────────────────────────────────────────────────────────────

func (a *App) pagePsychFinance(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/finance", basePsychData("finance"))
}

// ─── Settings ────────────────────────────────────────────────────────────────

func (a *App) pagePsychSettings(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/settings", basePsychData("settings"))
}
