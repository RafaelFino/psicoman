package web

import (
	"context"
	"net/http"
	"time"

	"github.com/fino/psicoman/internal/domain"
	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

func (a *App) registerPatientRoutes(r *gin.RouterGroup) {
	r.GET("/me", a.patientMe)
	r.GET("/appointments", a.patientAppointments)
	r.GET("/slots", a.patientSlots)
	r.POST("/appointments", a.patientCreateAppointment)
	r.PATCH("/appointments/:id/cancel", a.patientCancel)
	r.PATCH("/appointments/:id/reschedule", a.patientReschedule)
	r.PUT("/anamnesis", a.patientAnamnesis)
	r.GET("/documents", a.patientDocuments)
	r.POST("/documents", a.patientUploadDocument)
	r.GET("/documents/:id/download", a.downloadDocument)
}

func (a *App) registerPublicRoutes(r *gin.RouterGroup) {
	r.GET("/auth/patient/url", a.patientAuthURL)
	r.GET("/auth/patient/callback", a.patientAuthCallback)
	r.POST("/auth/patient/register", a.patientRegister)
}

func (a *App) patientAuthURL(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		state = "patient-login"
	}
	c.JSON(http.StatusOK, gin.H{"url": a.Auth.PatientAuthURL(state)})
}

func (a *App) patientAuthCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code ausente"})
		return
	}

	token, err := a.Auth.PatientOAuthConfig().Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_token ausente"})
		return
	}

	payload, err := idtoken.Validate(context.Background(), idToken, a.Config.GoogleClientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token google inválido"})
		return
	}

	email, _ := payload.Claims["email"].(string)
	sub, _ := payload.Claims["sub"].(string)
	name, _ := payload.Claims["name"].(string)

	db, _ := a.dbForTenant(a.Config.DefaultTenantID)
	patient, err := db.GetPatientByGoogleSub(sub)
	if err != nil {
		patient, err = db.GetPatientByEmail(email)
		if err != nil {
			patient, err = db.CreatePatient(domain.Patient{Email: email, Name: name, GoogleSub: sub})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			_ = db.UpdatePatientGoogleSub(patient.ID, sub)
		}
	}

	jwt, err := a.Auth.IssuePatientToken(patient.ID, patient.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/patient/login?token="+jwt)
}

func (a *App) patientRegister(c *gin.Context) {
	db, err := a.dbForTenant(a.Config.DefaultTenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var in service.RegisterPatientInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := a.Patient.Register(db, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (a *App) patientMe(c *gin.Context) {
	p, err := a.Patient.Get(getDB(c), c.GetString("patient_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paciente não encontrado"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (a *App) patientAppointments(c *gin.Context) {
	from, to := parseDateRange(c)
	list, err := a.Appt.List(getDB(c), from, to, c.GetString("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) patientSlots(c *gin.Context) {
	from, to := parseDateRange(c)
	slots, err := a.Appt.AvailableSlots(getDB(c), from, to, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, slots)
}

func (a *App) patientCreateAppointment(c *gin.Context) {
	var in struct {
		Type            domain.AppointmentType `json:"type"`
		ScheduledAt     time.Time            `json:"scheduled_at"`
		DurationMinutes int                  `json:"duration_minutes"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.setCalendarDB(c)
	appt, err := a.Appt.Create(c.Request.Context(), getDB(c), service.CreateAppointmentInput{
		PatientID: c.GetString("patient_id"), Type: in.Type,
		ScheduledAt: in.ScheduledAt, DurationMinutes: in.DurationMinutes,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, appt)
}

func (a *App) patientCancel(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	a.setCalendarDB(c)
	if err := a.Appt.Cancel(c.Request.Context(), getDB(c), c.Param("id"), body.Reason, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) patientReschedule(c *gin.Context) {
	var body struct {
		ScheduledAt time.Time `json:"scheduled_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.setCalendarDB(c)
	appt, err := a.Appt.Reschedule(c.Request.Context(), getDB(c), c.Param("id"), body.ScheduledAt, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, appt)
}

func (a *App) patientAnamnesis(c *gin.Context) {
	var body struct {
		Anamnesis string `json:"anamnesis"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Patient.UpdateAnamnesis(getDB(c), c.GetString("patient_id"), body.Anamnesis); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) patientDocuments(c *gin.Context) {
	docs, err := a.GED.List(getDB(c), c.GetString("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (a *App) patientUploadDocument(c *gin.Context) {
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
		PatientID:  c.GetString("patient_id"),
		Filename:   file.Filename,
		MimeType:   file.Header.Get("Content-Type"),
		UploadedBy: domain.UploadedByPatient,
		DocType:    domain.DocOutro,
	}
	saved, err := a.GED.Save(getDB(c), getTenant(c), doc, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}
