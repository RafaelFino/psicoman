package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *App) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), a.RequestLogger())

	api := r.Group("/api")
	a.registerPublicRoutes(api)

	psych := api.Group("/psych", a.StaffAuth())
	a.registerPsychRoutes(psych)

	patient := api.Group("/patient", a.PatientAuth())
	a.registerPatientRoutes(patient)

	// Dev-only helpers — only registered when DEV_MODE=true.
	if a.Config.DevMode {
		a.Log.Warn().Msg("DEV_MODE enabled — dev routes active at /api/dev/*. DO NOT use in production.")
		dev := api.Group("/dev")
		a.registerDevRoutes(dev)
	}

	a.serveFrontend(r)
	return r
}

func (a *App) serveFrontend(r *gin.Engine) {
	fs := frontendFS()
	if fs == nil {
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "Psicoman API. Build: make build")
		})
		return
	}

	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS("assets/"+c.Param("filepath"), fs)
	})
	spa := func(c *gin.Context) { c.FileFromFS("index.html", fs) }
	r.GET("/", spa)
	r.GET("/psych", spa)
	r.GET("/psych/*any", spa)
	r.GET("/patient", spa)
	r.GET("/patient/*any", spa)
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		spa(c)
	})
}
