package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
		Auth:        &service.AuthService{JWTSecret: cfg.JWTSecret},
		Patient:     &service.PatientService{},
		Appt:        &service.AppointmentService{Calendar: calendar},
		GED:         &service.GEDService{BaseDir: filepath.Join(dir, "ged")},
		Finance:     &service.FinanceService{},
		SessionNote: &service.SessionNoteService{},
		Anamnesis:   &service.AnamnesisService{},
		Contract:    &service.ContractService{},
		Supervision: &service.SupervisionService{},
		Space:       &service.SpaceService{},
		Tmpl:        NewTemplateRenderer(),
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

func TestRootRedirectsToPsych(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := testApp(t)
	r := app.Router()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/psych", w.Header().Get("Location"))
}

func TestPatientDetailCalendarAPI(t *testing.T) {
	app := testApp(t)
	app.Config.DevMode = true
	r := app.Router()

	// 1. Create patient
	body, _ := json.Marshal(map[string]string{"name": "TEST CalendarPat", "email": "test.calendar@test.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/psych/patients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var patient map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &patient)
	patientID := patient["id"].(string)
	assert.NotEmpty(t, patientID)

	// 2. Create appointment
	futureTime := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	apptBody, _ := json.Marshal(map[string]interface{}{
		"patient_id":       patientID,
		"type":             "online",
		"scheduled_at":     futureTime.Format(time.RFC3339),
		"duration_minutes": 50,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/psych/appointments", bytes.NewReader(apptBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var appt map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &appt)
	assert.Equal(t, patientID, appt["patient_id"])

	// 3. List appointments with patient_id filter
	from := time.Now().UTC().Truncate(24 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	req = httptest.NewRequest(http.MethodGet, "/api/psych/appointments?from="+from+"&to="+to+"&patient_id="+patientID, nil)
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var appts []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &appts)
	assert.GreaterOrEqual(t, len(appts), 1)
	assert.Equal(t, patientID, appts[0]["patient_id"])
}

func TestDevDeleteTestData(t *testing.T) {
	app := testApp(t)
	app.Config.DevMode = true
	app.Config.DevSecret = "dev-local"
	r := app.Router()

	// Create a TEST patient
	body, _ := json.Marshal(map[string]string{"name": "TEST DeleteMe", "email": "test.delete@test.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/psych/patients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Create a normal patient (should NOT be deleted)
	body, _ = json.Marshal(map[string]string{"name": "Normal Patient", "email": "normal@example.com"})
	req = httptest.NewRequest(http.MethodPost, "/api/psych/patients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Call DELETE /api/dev/test-data with valid auth
	req = httptest.NewRequest(http.MethodDelete, "/api/dev/test-data", nil)
	req.Header.Set("X-Dev-Auth", "dev-local")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var counts map[string]int
	json.Unmarshal(w.Body.Bytes(), &counts)
	assert.Equal(t, 1, counts["patients"])

	// Verify normal patient still exists
	req = httptest.NewRequest(http.MethodGet, "/api/psych/patients", nil)
	req.Header.Set("X-User-Id", "default")
	req.Header.Set("X-User-Email", "psych@local")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var patients []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &patients)
	assert.Equal(t, 1, len(patients))
	assert.Equal(t, "Normal Patient", patients[0]["name"])

	// Test 401 without auth
	req = httptest.NewRequest(http.MethodDelete, "/api/dev/test-data", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
