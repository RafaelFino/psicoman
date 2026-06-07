package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/fino/psicoman/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		Addr: ":0", DataDir: dir,
		JWTSecret: "test-secret",
		DefaultTenantID: "default",
		PangolinUserHeader: "X-User-Id",
		PangolinEmailHeader: "X-User-Email",
		PangolinRoleHeader: "X-User-Role",
	}
	calendar := &service.DBCalendar{Noop: &service.NoopCalendar{}}
	return &App{
		Config: cfg, Log: NewLogger(dir),
		Auth: &service.AuthService{JWTSecret: cfg.JWTSecret},
		Patient: &service.PatientService{},
		Appt: &service.AppointmentService{Calendar: calendar},
		GED: &service.GEDService{BaseDir: filepath.Join(dir, "ged")},
		Finance: &service.FinanceService{},
	}
}

func TestCreatePatientPsych(t *testing.T) {
	app := testApp(t)
	r := app.Router()

	body, _ := json.Marshal(map[string]string{"name": "Test", "email": "test@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/psych/patients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPatientRegisterPublic(t *testing.T) {
	app := testApp(t)
	r := app.Router()

	body, _ := json.Marshal(map[string]string{"name": "Paciente", "email": "pac@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/patient/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPatientJWTAuth(t *testing.T) {
	app := testApp(t)
	token, err := app.Auth.IssuePatientToken("patient-1", "p@example.com")
	require.NoError(t, err)

	r := app.Router()
	req := httptest.NewRequest(http.MethodGet, "/api/patient/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code) // patient not in db
}

func TestHealthWithoutFrontend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := testApp(t)
	r := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
