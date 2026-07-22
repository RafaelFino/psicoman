package web

import (
	"net/http"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── Psych: Anamnesis Templates ──────────────────────────────────────────────

func (a *App) listAnamnesisTemplates(c *gin.Context) {
	list, err := a.Anamnesis.ListTemplates(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createAnamnesisTemplate(c *gin.Context) {
	var in service.CreateAnamnesisTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := a.Anamnesis.CreateTemplate(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (a *App) updateAnamnesisTemplate(c *gin.Context) {
	var in service.UpdateAnamnesisTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := a.Anamnesis.UpdateTemplate(getDB(c), c.Param("id"), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (a *App) deleteAnamnesisTemplate(c *gin.Context) {
	if err := a.Anamnesis.DeleteTemplate(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listAnamnesisResponses(c *gin.Context) {
	list, err := a.Anamnesis.ListResponses(getDB(c), c.Query("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) getAnamnesisResponse(c *gin.Context) {
	r, err := a.Anamnesis.GetResponse(getDB(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "resposta não encontrada"})
		return
	}
	c.JSON(http.StatusOK, r)
}

// ─── Patient: Anamnesis submission ───────────────────────────────────────────

func (a *App) patientListAnamnesisTemplates(c *gin.Context) {
	list, err := a.Anamnesis.ListTemplates(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Filter to only active templates
	var active []any
	for _, t := range list {
		if t.IsActive {
			active = append(active, t)
		}
	}
	c.JSON(http.StatusOK, active)
}

func (a *App) patientSubmitAnamnesis(c *gin.Context) {
	var in service.SubmitAnamnesisInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := a.Anamnesis.Submit(getDB(c), c.GetString("patient_id"), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r)
}

func (a *App) patientListAnamnesisResponses(c *gin.Context) {
	list, err := a.Anamnesis.ListResponses(getDB(c), c.GetString("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}
