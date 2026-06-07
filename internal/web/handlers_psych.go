package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

func (a *App) registerPsychRoutes(r *gin.RouterGroup) {
	r.GET("/me", a.psychMe)
	r.GET("/patients", a.listPatients)
	r.POST("/patients", a.createPatient)
	r.GET("/patients/:id", a.getPatient)
	r.GET("/patients/:id/report", a.patientReport)

	r.GET("/appointments", a.listAppointments)
	r.POST("/appointments", a.createAppointment)
	r.PATCH("/appointments/:id/cancel", a.cancelAppointment)
	r.PATCH("/appointments/:id/reschedule", a.rescheduleAppointment)
	r.PATCH("/appointments/:id/notes", a.updateNotes)
	r.PATCH("/appointments/:id/complete", a.completeAppointment)

	r.GET("/scheduling-rules", a.getRules)
	r.PUT("/scheduling-rules", a.updateRules)

	r.GET("/documents", a.listDocumentsPsych)
	r.POST("/documents", a.uploadDocumentPsych)
	r.GET("/documents/:id/download", a.downloadDocument)

	r.GET("/finance/summary", a.financeSummary)
	r.GET("/finance/reports/monthly", a.monthlyReports)
	r.POST("/finance/payments", a.addPayment)
	r.POST("/finance/payments/:id/receive", a.receivePayment)
	r.POST("/finance/costs", a.addCost)

	r.GET("/google/auth", a.googleAuthURL)
	r.GET("/google/callback", a.googleCallback)
}

func (a *App) psychMe(c *gin.Context) {
	staff := c.MustGet("staff")
	c.JSON(http.StatusOK, staff)
}

func (a *App) listPatients(c *gin.Context) {
	db := getDB(c)
	list, err := a.Patient.List(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createPatient(c *gin.Context) {
	var in service.RegisterPatientInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := a.Patient.Register(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (a *App) getPatient(c *gin.Context) {
	p, err := a.Patient.Get(getDB(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (a *App) patientReport(c *gin.Context) {
	report, err := a.Patient.FullReport(getDB(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func parseDateRange(c *gin.Context) (time.Time, time.Time) {
	from := time.Now().UTC().Truncate(24 * time.Hour)
	to := from.AddDate(0, 1, 0)
	if f := c.Query("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = t
		}
	}
	if t := c.Query("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		}
	}
	return from, to
}

func (a *App) listAppointments(c *gin.Context) {
	from, to := parseDateRange(c)
	list, err := a.Appt.List(getDB(c), from, to, c.Query("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createAppointment(c *gin.Context) {
	var in struct {
		PatientID       string                `json:"patient_id"`
		Type            domain.AppointmentType `json:"type"`
		ScheduledAt     time.Time             `json:"scheduled_at"`
		DurationMinutes int                   `json:"duration_minutes"`
		Notes           string                `json:"notes"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a.setCalendarDB(c)
	appt, err := a.Appt.Create(c.Request.Context(), getDB(c), service.CreateAppointmentInput{
		PatientID: in.PatientID, Type: in.Type, ScheduledAt: in.ScheduledAt,
		DurationMinutes: in.DurationMinutes, Notes: in.Notes,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, appt)
}

func (a *App) cancelAppointment(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	a.setCalendarDB(c)
	if err := a.Appt.Cancel(c.Request.Context(), getDB(c), c.Param("id"), body.Reason, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) rescheduleAppointment(c *gin.Context) {
	var body struct {
		ScheduledAt time.Time `json:"scheduled_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.setCalendarDB(c)
	appt, err := a.Appt.Reschedule(c.Request.Context(), getDB(c), c.Param("id"), body.ScheduledAt, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, appt)
}

func (a *App) updateNotes(c *gin.Context) {
	var body struct {
		Notes      string `json:"notes"`
		ReportHTML string `json:"report_html"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	appt, err := a.Appt.UpdateNotes(getDB(c), c.Param("id"), body.Notes, body.ReportHTML)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, appt)
}

func (a *App) completeAppointment(c *gin.Context) {
	appt, err := a.Appt.Complete(getDB(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, appt)
}

func (a *App) getRules(c *gin.Context) {
	rules, err := getDB(c).GetSchedulingRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *App) updateRules(c *gin.Context) {
	var rules domain.SchedulingRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := getDB(c).UpdateSchedulingRules(rules); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *App) listDocumentsPsych(c *gin.Context) {
	docs, err := a.GED.List(getDB(c), c.Query("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (a *App) uploadDocumentPsych(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	doc := domain.Document{
		PatientID:     c.PostForm("patient_id"),
		AppointmentID: c.PostForm("appointment_id"),
		Filename:      file.Filename,
		MimeType:      file.Header.Get("Content-Type"),
		UploadedBy:    domain.UploadedByPsychologist,
		DocType:       domain.DocType(c.DefaultPostForm("doc_type", string(domain.DocOutro))),
	}
	saved, err := a.GED.Save(getDB(c), getTenant(c), doc, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}

func (a *App) downloadDocument(c *gin.Context) {
	db := getDB(c)
	doc, err := db.GetDocument(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "documento não encontrado"})
		return
	}
	if pid, ok := c.Get("patient_id"); ok && doc.PatientID != pid.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "acesso negado"})
		return
	}
	f, err := a.GED.Open(*doc)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	c.Header("Content-Disposition", "attachment; filename="+doc.Filename)
	c.Header("Content-Type", doc.MimeType)
	c.File(doc.Path)
}

func (a *App) financeSummary(c *gin.Context) {
	month, year := parseMonthYear(c)
	summary, err := a.Finance.Summary(getDB(c), month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (a *App) monthlyReports(c *gin.Context) {
	month, year := parseMonthYear(c)
	reports, err := a.Finance.MonthlyReports(getDB(c), month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reports)
}

func (a *App) addPayment(c *gin.Context) {
	var p domain.Payment
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := a.Finance.AddPayment(getDB(c), p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}

func (a *App) receivePayment(c *gin.Context) {
	if err := a.Finance.ReceivePayment(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) addCost(c *gin.Context) {
	var cost domain.Cost
	if err := c.ShouldBindJSON(&cost); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := a.Finance.AddCost(getDB(c), cost)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}

func (a *App) googleAuthURL(c *gin.Context) {
	if a.Google == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "google não configurado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": a.Google.AuthURL("psych-google")})
}

func (a *App) googleCallback(c *gin.Context) {
	if a.Google == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "google não configurado"})
		return
	}
	code := c.Query("code")
	token, err := a.Google.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Google.SaveToken(getDB(c), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/psych/settings?google=connected")
}

func parseMonthYear(c *gin.Context) (int, int) {
	now := time.Now().UTC()
	month := int(now.Month())
	year := now.Year()
	if m := c.Query("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			month = v
		}
	}
	if y := c.Query("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	return month, year
}

func (a *App) setCalendarDB(c *gin.Context) {
	if cal, ok := a.Appt.Calendar.(*service.DBCalendar); ok {
		cal.DB = getDB(c)
	}
}
