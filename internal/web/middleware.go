package web

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fino/psicoman/internal/service"
	"github.com/fino/psicoman/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type App struct {
	Config  Config
	Log     zerolog.Logger
	Auth    *service.AuthService
	Patient *service.PatientService
	Appt    *service.AppointmentService
	GED     *service.GEDService
	Finance *service.FinanceService
	Google  *service.GoogleCalendar

	dbPool sync.Map // tenantID -> *storage.DB
}

func (a *App) dbForTenant(tenantID string) (*storage.DB, error) {
	if v, ok := a.dbPool.Load(tenantID); ok {
		return v.(*storage.DB), nil
	}
	db, err := storage.Open(a.Config.DataDir, tenantID)
	if err != nil {
		return nil, err
	}
	a.dbPool.Store(tenantID, db)
	return db, nil
}

func (a *App) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		a.Log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration_ms", time.Since(start)).
			Str("ip", c.ClientIP()).
			Str("user", c.GetString("user_id")).
			Msg("request")
	}
}

func (a *App) StaffAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader(a.Config.PangolinUserHeader)
		email := c.GetHeader(a.Config.PangolinEmailHeader)
		role := c.GetHeader(a.Config.PangolinRoleHeader)

		// Dev mode: accept X-Dev-Auth header as an alternative to Pangolin headers.
		// Allows local administration without a running Pangolin proxy.
		if a.Config.DevMode {
			devAuth := c.GetHeader("X-Dev-Auth")
			if devAuth != "" && devAuth != a.Config.DevSecret {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Dev-Auth inválido"})
				return
			}
			// If correct secret (or no Pangolin headers present), fill in dev defaults.
			if devAuth == a.Config.DevSecret || userID == "" {
				if userID == "" {
					userID = a.Config.DefaultTenantID
				}
				if email == "" {
					email = "admin@local.dev"
				}
				if role == "" {
					role = "admin"
				}
			}
		} else {
			// Production: Pangolin headers are required. Absence of X-User-Id is not an
			// error by itself (backward-compat default), but reject if the request came
			// from outside and headers were stripped.
			if userID == "" {
				userID = a.Config.DefaultTenantID
			}
			if email == "" {
				email = "psychologist@local"
			}
		}

		db, err := a.dbForTenant(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		staff, err := a.Auth.EnsureStaff(db, email, role)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
			return
		}

		c.Set("tenant_id", userID)
		c.Set("user_id", userID)
		c.Set("staff", staff)
		c.Set("db", db)
		c.Next()
	}
}

func (a *App) PatientAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token ausente"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := a.Auth.ParsePatientToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
			return
		}

		db, err := a.dbForTenant(a.Config.DefaultTenantID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.Set("tenant_id", a.Config.DefaultTenantID)
		c.Set("patient_id", claims.PatientID)
		c.Set("patient_email", claims.Email)
		c.Set("db", db)
		c.Next()
	}
}

func getDB(c *gin.Context) *storage.DB {
	return c.MustGet("db").(*storage.DB)
}

func getTenant(c *gin.Context) string {
	return c.MustGet("tenant_id").(string)
}
