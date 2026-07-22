package web

import (
	"net/http"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

func (a *App) registerSpaceRoutes(r *gin.RouterGroup) {
	r.GET("/spaces", a.listSpaces)
	r.POST("/spaces", a.createSpace)
	r.PUT("/spaces/:id", a.updateSpace)
	r.DELETE("/spaces/:id", a.deleteSpace)

	r.GET("/space-bookings", a.listSpaceBookings)
	r.POST("/space-bookings", a.createSpaceBooking)
	r.DELETE("/space-bookings/:id", a.deleteSpaceBooking)
}

func (a *App) listSpaces(c *gin.Context) {
	list, err := a.Space.ListSpaces(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createSpace(c *gin.Context) {
	var in service.CreateSpaceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := a.Space.CreateSpace(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (a *App) updateSpace(c *gin.Context) {
	var in service.CreateSpaceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Space.UpdateSpace(getDB(c), c.Param("id"), in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) deleteSpace(c *gin.Context) {
	if err := a.Space.DeleteSpace(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listSpaceBookings(c *gin.Context) {
	list, err := a.Space.ListBookings(getDB(c), c.Query("space_id"), c.Query("date"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createSpaceBooking(c *gin.Context) {
	var in service.CreateSpaceBookingInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b, err := a.Space.CreateBooking(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (a *App) deleteSpaceBooking(c *gin.Context) {
	if err := a.Space.DeleteBooking(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
