package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *App) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), a.RequestLogger(), a.SecurityHeaders())

	// Static assets (CSS, JS — htmx, Alpine)
	ServeStatic(r)

	// ─── API Routes (JSON) ───────────────────────────────────────────────────
	api := r.Group("/api")
	a.registerPublicRoutes(api)

	psych := api.Group("/psych", a.StaffAuth())
	a.registerPsychRoutes(psych)

	patient := api.Group("/patient", a.PatientAuth())
	a.registerPatientRoutes(patient)

	// Dev-only helpers
	if a.Config.DevMode {
		a.Log.Warn().Msg("DEV_MODE enabled — dev routes active at /api/dev/*. DO NOT use in production.")
		dev := api.Group("/dev")
		a.registerDevRoutes(dev)
	}

	// ─── Page Routes (HTML templates) ────────────────────────────────────────
	psychPages := r.Group("/psych", a.StaffAuth())
	a.registerPsychPages(psychPages)

	// Patient pages (requires JWT cookie or token)
	patientPages := r.Group("/patient", a.PatientPageAuth())
	a.registerPatientPages(patientPages)

	// Public pages
	r.GET("/patient/login", a.pagePatientLogin)
	r.GET("/patient/logout", a.pagePatientLogout)

	// Root redirect
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/psych")
	})

	// Catch-all: if it's not /api, show 404 page
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.String(http.StatusNotFound, "Página não encontrada")
	})

	return r
}

// SecurityHeaders adds standard security headers to all responses.
func (a *App) SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}

// PatientPageAuth is like PatientAuth but reads token from cookie or query param.
func (a *App) PatientPageAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try cookie first
		token, err := c.Cookie("patient_token")
		if err != nil || token == "" {
			// Try Authorization header (for htmx requests)
			header := c.GetHeader("Authorization")
			if header != "" && len(header) > 7 {
				token = header[7:] // Strip "Bearer "
			}
		}
		if token == "" {
			c.Redirect(http.StatusFound, "/patient/login")
			c.Abort()
			return
		}

		claims, err := a.Auth.ParsePatientToken(token)
		if err != nil {
			c.Redirect(http.StatusFound, "/patient/login")
			c.Abort()
			return
		}

		db, err := a.dbForTenant(a.Config.DefaultTenantID)
		if err != nil {
			c.String(http.StatusInternalServerError, "database error")
			c.Abort()
			return
		}

		c.Set("tenant_id", a.Config.DefaultTenantID)
		c.Set("patient_id", claims.PatientID)
		c.Set("patient_email", claims.Email)
		c.Set("db", db)
		c.Next()
	}
}
