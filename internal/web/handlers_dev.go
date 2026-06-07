package web

// Dev-mode handlers — only registered when DEV_MODE=true.
// These routes allow local testing without Pangolin or Google OAuth.
// Never enable DEV_MODE in production.

import (
	"net/http"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

func (a *App) registerDevRoutes(r *gin.RouterGroup) {
	// GET /api/dev/status — confirms dev mode is active and shows current config.
	r.GET("/status", a.devStatus)

	// POST /api/dev/patient-token — issues a patient JWT for any patient_id / email.
	// Body: {"patient_id": "...", "email": "..."}
	// Requires X-Dev-Auth header with the configured DEV_SECRET.
	r.POST("/patient-token", a.devPatientToken)

	// POST /api/dev/create-patient — creates a patient directly and returns a JWT.
	// Useful for bootstrapping a fresh database without going through Google OAuth.
	// Body: {"name": "...", "email": "..."}
	// Requires X-Dev-Auth header.
	r.POST("/create-patient", a.devCreatePatient)
}

func (a *App) requireDevSecret(c *gin.Context) bool {
	secret := c.GetHeader("X-Dev-Auth")
	if secret != a.Config.DevSecret {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Dev-Auth inválido ou ausente"})
		return false
	}
	return true
}

func (a *App) devStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"dev_mode":          true,
		"default_tenant_id": a.Config.DefaultTenantID,
		"addr":              a.Config.Addr,
		"data_dir":          a.Config.DataDir,
		"google_configured": a.Config.GoogleClientID != "",
		"hint_psych":        "Add header X-Dev-Auth: <DEV_SECRET> to any /api/psych/* request",
		"hint_patient":      "POST /api/dev/create-patient to get a patient JWT",
	})
}

func (a *App) devPatientToken(c *gin.Context) {
	if !a.requireDevSecret(c) {
		return
	}
	var body struct {
		PatientID string `json:"patient_id" binding:"required"`
		Email     string `json:"email"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := a.Auth.IssuePatientToken(body.PatientID, body.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (a *App) devCreatePatient(c *gin.Context) {
	if !a.requireDevSecret(c) {
		return
	}
	var body struct {
		Name  string `json:"name"  binding:"required"`
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := a.dbForTenant(a.Config.DefaultTenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error: " + err.Error()})
		return
	}

	// Reuse existing patient if email already registered.
	patient, err := db.GetPatientByEmail(body.Email)
	if err != nil {
		// Not found — create.
		from, regErr := a.Patient.Register(db, service.RegisterPatientInput{
			Name:  body.Name,
			Email: body.Email,
		})
		if regErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": regErr.Error()})
			return
		}
		patient = from
	}

	token, err := a.Auth.IssuePatientToken(patient.ID, patient.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"patient": patient,
		"token":   token,
	})
}
