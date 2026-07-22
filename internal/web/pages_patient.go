package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerPatientPages registers HTML page routes for the patient interface.
func (a *App) registerPatientPages(r *gin.RouterGroup) {
	r.GET("", a.pagePatientDashboard)
	r.GET("/book", a.pagePatientBook)
	r.GET("/anamnesis", a.pagePatientAnamnesis)
	r.GET("/contracts", a.pagePatientContracts)
	r.GET("/documents", a.pagePatientDocuments)
}

func (a *App) pagePatientLogin(c *gin.Context) {
	// Check if already logged in
	if token, _ := c.Cookie("patient_token"); token != "" {
		if _, err := a.Auth.ParsePatientToken(token); err == nil {
			c.Redirect(http.StatusFound, "/patient")
			return
		}
	}

	data := struct {
		PageData
		DevMode       bool
		GoogleAuthURL string
	}{
		PageData: basePatientData(""),
		DevMode:  a.Config.DevMode,
	}
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/login", data)
}

func (a *App) pagePatientLogout(c *gin.Context) {
	c.SetCookie("patient_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/patient/login")
}

func (a *App) pagePatientDashboard(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/dashboard", basePatientData("dashboard"))
}

func (a *App) pagePatientBook(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/book", basePatientData("book"))
}

func (a *App) pagePatientAnamnesis(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/anamnesis", basePatientData("anamnesis"))
}

func (a *App) pagePatientContracts(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/contracts", basePatientData("contracts"))
}

func (a *App) pagePatientDocuments(c *gin.Context) {
	a.Tmpl.RenderPage(c, http.StatusOK, "patient/documents", basePatientData("documents"))
}
