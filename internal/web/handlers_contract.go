package web

import (
	"net/http"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
)

// ─── Psych: Contract Templates & Contracts ───────────────────────────────────

func (a *App) listContractTemplates(c *gin.Context) {
	list, err := a.Contract.ListTemplates(getDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createContractTemplate(c *gin.Context) {
	var in service.CreateContractTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := a.Contract.CreateTemplate(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (a *App) updateContractTemplate(c *gin.Context) {
	var in service.CreateContractTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Contract.UpdateTemplate(getDB(c), c.Param("id"), in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listContracts(c *gin.Context) {
	list, err := a.Contract.ListContracts(getDB(c), c.Query("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) createContract(c *gin.Context) {
	var in service.CreateContractInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ct, err := a.Contract.Create(getDB(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ct)
}

func (a *App) revokeContract(c *gin.Context) {
	if err := a.Contract.Revoke(getDB(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ─── Patient: Contract viewing & signing ─────────────────────────────────────

func (a *App) patientListContracts(c *gin.Context) {
	list, err := a.Contract.ListContracts(getDB(c), c.GetString("patient_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (a *App) patientSignContract(c *gin.Context) {
	patientID := c.GetString("patient_id")
	contractID := c.Param("id")
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	if err := a.Contract.Sign(getDB(c), contractID, patientID, ip, ua); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Contrato assinado com sucesso"})
}
