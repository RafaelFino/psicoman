package web

import (
	"io"
	iofs "io/fs"
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
	fsys := frontendFS()
	if fsys == nil {
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "Psicoman API. Build: make build")
		})
		return
	}

	// Sub-filesystem rooted at assets/ so http.FileServer serves with correct
	// Content-Type and no path prefix issues.
	assetsSub, err := iofs.Sub(fsys, "assets")
	if err != nil {
		a.Log.Error().Err(err).Msg("failed to create assets sub-filesystem")
		return
	}
	assetsServer := http.FileServer(http.FS(assetsSub))

	r.GET("/assets/*filepath", func(c *gin.Context) {
		// Strip "/assets" prefix so FileServer resolves relative to assetsSub root.
		c.Request.URL.Path = c.Param("filepath")
		assetsServer.ServeHTTP(c.Writer, c.Request)
	})

	// SPA handler: serves index.html directly to avoid 301 redirect loops
	// that occur with c.FileFromFS when using embed.FS.
	spa := func(c *gin.Context) {
		f, err := fsys.Open("index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusOK)
		io.Copy(c.Writer, f.(iofs.File))
	}

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
