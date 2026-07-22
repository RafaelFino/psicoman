package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/gin-gonic/gin"
)

// registerPsychPages registers HTML page routes for the psych interface.
func (a *App) registerPsychPages(r *gin.RouterGroup) {
	r.GET("", a.pagePsychDashboard)
	r.GET("/patients", a.pagePsychPatients)
	r.GET("/patients/:id", a.pagePsychPatientDetail)
	r.GET("/patients/:id/calendar", a.pagePsychPatientCalendar)
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

// CalendarDay represents one cell in the monthly calendar grid.
type CalendarDay struct {
	Day          int                  // 0 = padding cell (outside month)
	Date         string               // "2026-07-22" format
	IsToday      bool
	Appointments []domain.Appointment
}

// CalendarMonth holds the full month grid data for rendering.
type CalendarMonth struct {
	Year      int
	Month     int
	MonthName string
	Days      []CalendarDay
	PrevMonth string // "?year=2026&month=6" format for URL
	NextMonth string // "?year=2026&month=8" format for URL
	PatientID string
}

// buildCalendarMonth produces the calendar data for a given month/year and patient's appointments.
func buildCalendarMonth(year, month int, patientID string, appointments []domain.Appointment) CalendarMonth {
	// Portuguese month names
	monthNames := []string{"", "Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
		"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}

	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1).Day()
	weekdayOffset := int(firstDay.Weekday()) // Sunday=0

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Total cells = padding + days in month, rounded up to complete weeks
	totalCells := weekdayOffset + lastDay
	if totalCells%7 != 0 {
		totalCells += 7 - (totalCells % 7)
	}

	days := make([]CalendarDay, totalCells)

	// Fill padding cells (Day=0)
	for i := 0; i < weekdayOffset; i++ {
		days[i] = CalendarDay{Day: 0}
	}

	// Fill actual days
	for d := 1; d <= lastDay; d++ {
		idx := weekdayOffset + d - 1
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
		cd := CalendarDay{
			Day:     d,
			Date:    date.Format("2006-01-02"),
			IsToday: date.Equal(today),
		}
		// Attach appointments for this day
		for _, appt := range appointments {
			apptDate := time.Date(appt.ScheduledAt.Year(), appt.ScheduledAt.Month(), appt.ScheduledAt.Day(), 0, 0, 0, 0, time.UTC)
			if apptDate.Equal(date) {
				cd.Appointments = append(cd.Appointments, appt)
			}
		}
		days[idx] = cd
	}

	// Trailing padding cells
	for i := weekdayOffset + lastDay; i < totalCells; i++ {
		days[i] = CalendarDay{Day: 0}
	}

	// Compute prev/next month navigation
	prevYear, prevMonth := year, month-1
	if prevMonth < 1 {
		prevMonth = 12
		prevYear--
	}
	nextYear, nextMonth := year, month+1
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	return CalendarMonth{
		Year:      year,
		Month:     month,
		MonthName: fmt.Sprintf("%s %d", monthNames[month], year),
		Days:      days,
		PrevMonth: fmt.Sprintf("?year=%d&month=%d", prevYear, prevMonth),
		NextMonth: fmt.Sprintf("?year=%d&month=%d", nextYear, nextMonth),
		PatientID: patientID,
	}
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

// PatientMetrics holds computed metrics for the patient detail page.
type PatientMetrics struct {
	PatientMinutes  int
	AnalysisMinutes int
	AdminMinutes    int
	TotalMinutes    int
	PatientPct      int
	AnalysisPct     int
	AdminPct        int
	PendingCents    int64
	ReceivedCents   int64
}

type patientDetailData struct {
	PageData
	Patient            *domain.Patient
	Calendar           CalendarMonth
	Metrics            PatientMetrics
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

	now := time.Now().UTC()

	// Load all related data (full history for lists)
	from := patient.CreatedAt
	to := now.AddDate(1, 0, 0)
	appts, _ := db.ListAppointments(from, to, id)
	notes, _ := db.ListSessionNotes(id)
	docs, _ := db.ListDocuments(id)
	contracts, _ := db.ListContracts(id)
	responses, _ := db.ListAnamnesisResponses(id)

	// Load payments for this patient (all months)
	var payments []domain.Payment
	for m := 1; m <= 12; m++ {
		monthly, _ := db.ListPayments(m, now.Year())
		for _, p := range monthly {
			if p.PatientID == id {
				payments = append(payments, p)
			}
		}
	}

	// Current month appointments for calendar
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	monthAppts, _ := db.ListAppointments(monthStart, monthEnd, id)

	// Build calendar
	calendar := buildCalendarMonth(now.Year(), int(now.Month()), id, monthAppts)

	// Metrics
	patientMin, analysisMin, adminMin, _ := db.SessionNoteHoursForPatientMonth(id, int(now.Month()), now.Year())
	pendingCents, receivedCents, _ := db.PatientPaymentSummary(id)
	totalMin := patientMin + analysisMin + adminMin
	var patientPct, analysisPct, adminPct int
	if totalMin > 0 {
		patientPct = patientMin * 100 / totalMin
		analysisPct = analysisMin * 100 / totalMin
		adminPct = adminMin * 100 / totalMin
	}
	metrics := PatientMetrics{
		PatientMinutes:  patientMin,
		AnalysisMinutes: analysisMin,
		AdminMinutes:    adminMin,
		TotalMinutes:    totalMin,
		PatientPct:      patientPct,
		AnalysisPct:     analysisPct,
		AdminPct:        adminPct,
		PendingCents:    pendingCents,
		ReceivedCents:   receivedCents,
	}

	data := patientDetailData{
		PageData:           basePsychData("patients"),
		Patient:            patient,
		Calendar:           calendar,
		Metrics:            metrics,
		Appointments:       appts,
		SessionNotes:       notes,
		Documents:          docs,
		Contracts:          contracts,
		Payments:           payments,
		AnamnesisResponses: responses,
	}
	a.Tmpl.RenderPage(c, http.StatusOK, "psych/patient_detail", data)
}

// ─── Patient Calendar Fragment (htmx) ────────────────────────────────────────

// pagePsychPatientCalendar returns only the calendar grid HTML for htmx month navigation.
func (a *App) pagePsychPatientCalendar(c *gin.Context) {
	db := getDB(c)
	id := c.Param("id")
	now := time.Now().UTC()

	year := now.Year()
	month := int(now.Month())

	if y := c.Query("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := c.Query("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			month = v
		}
	}

	// Normalize month overflow (e.g. month=0 or month=13)
	if month < 1 {
		month = 12
		year--
	} else if month > 12 {
		month = 1
		year++
	}

	// Fetch appointments for the requested month
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	appts, _ := db.ListAppointments(monthStart, monthEnd, id)

	calendar := buildCalendarMonth(year, month, id, appts)
	a.Tmpl.RenderPartial(c, http.StatusOK, "calendar_grid", calendar)
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
