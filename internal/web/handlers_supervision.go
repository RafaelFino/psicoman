package web

import (
	"net/http"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

func (a *App) registerSupervisionRoutes(r *gin.RouterGroup) {
	r.GET("/supervisors", a.listSupervisors)
	r.POST("/supervisors", a.createSupervisor)
	r.PUT("/supervisors/:id", a.updateSupervisor)
	r.DELETE("/supervisors/:id", a.deleteSupervisor)

	r.GET("/supervision-sessions", a.listSupervisionSessions)
	r.POST("/supervision-sessions", a.createSupervisionSession)
	r.PUT("/supervision-sessions/:id", a.updateSupervisionSession)
	r.GET("/supervision-sessions/hours", a.supervisionHours)
}

func (a *App) listSupervisors(c *gin.Context) {
	list, err := a.Supervision.ListSupervisors(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createSupervisor(c *gin.Context) {
	var in service.CreateSupervisorInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := a.Supervision.CreateSupervisor(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (a *App) updateSupervisor(c *gin.Context) {
	var in service.CreateSupervisorInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Supervision.UpdateSupervisor(getDB(c), c.Param("id"), in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) deleteSupervisor(c *gin.Context) {
	if err := a.Supervision.DeleteSupervisor(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listSupervisionSessions(c *gin.Context) {
	list, err := a.Supervision.ListSessions(getDB(c), c.Query("supervisor_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createSupervisionSession(c *gin.Context) {
	var in service.CreateSupervisionSessionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ss, err := a.Supervision.CreateSession(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ss)
}

func (a *App) updateSupervisionSession(c *gin.Context) {
	var in service.UpdateSupervisionSessionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ss, err := a.Supervision.UpdateSession(getDB(c), c.Param("id"), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ss)
}

func (a *App) supervisionHours(c *gin.Context) {
	month, year := parseMonthYear(c)
	hours, err := a.Supervision.MonthlyHours(getDB(c), month, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"supervision_minutes": hours})
}
