# Design Document — Patient Detail Calendar

## Architecture Overview

This feature evolves the existing patient detail page (`/psych/patients/:id`) into a full-featured patient 360° view with a monthly calendar, tab navigation, metrics panel, and supporting test infrastructure.

The implementation stays within the existing monolith architecture:
- **Backend**: Go (Gin), SQLite, Go `html/template` + `embed`
- **Frontend**: htmx 2.x for dynamic content, Alpine.js 3.x for client-side state, CSS custom for styling
- **No new Go dependencies** — all functionality uses stdlib + existing packages

### Layer Responsibilities

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser (htmx + Alpine.js)                                     │
│  - Tab navigation (Alpine x-data/x-show)                        │
│  - Calendar month navigation (htmx hx-get + hx-swap)            │
│  - Modal interactions (Alpine.js component)                      │
│  - Metrics display (CSS progress bars / SVG)                     │
└────────────────────────────────────┬────────────────────────────┘
                                     │ HTTP (HTML fragments + JSON API)
┌────────────────────────────────────▼────────────────────────────┐
│  internal/web                                                    │
│  - pages_psych.go: pagePsychPatientDetail (full page render)     │
│  - pages_psych.go: htmx fragment handlers (calendar partial)     │
│  - handlers_dev.go: DELETE /api/dev/test-data                    │
│  - templates/psych/patient_detail.html (main template)           │
│  - templates/psych/partials/calendar_grid.html (htmx partial)    │
└────────────────────────────────────┬────────────────────────────┘
                                     │
┌────────────────────────────────────▼────────────────────────────┐
│  internal/service                                                │
│  - PatientService (existing)                                     │
│  - AppointmentService (existing)                                 │
│  - SessionNoteService.PatientMonthlyHours (new method)           │
│  - FinanceService.PatientSummary (new method)                    │
└────────────────────────────────────┬────────────────────────────┘
                                     │
┌────────────────────────────────────▼────────────────────────────┐
│  internal/storage                                                │
│  - DB.SessionNoteHoursForPatientMonth (new query)                │
│  - DB.PatientPaymentSummary (new query)                          │
│  - DB.DeleteTestData (new — cascading delete for TEST* patients) │
└─────────────────────────────────────────────────────────────────┘
```

---

## Components

### 1. Calendar Data Builder (Go — pure logic)

A utility function in `internal/web/pages_psych.go` that builds the calendar grid data structure for template rendering.

```go
// CalendarDay represents one cell in the monthly calendar grid.
type CalendarDay struct {
    Day          int                  // 0 = padding cell (outside month)
    Date         string               // "2026-07-22" format for htmx/Alpine
    IsToday      bool
    Appointments []domain.Appointment // appointments on this day
}

// CalendarMonth holds the full month grid data for rendering.
type CalendarMonth struct {
    Year       int
    Month      int
    MonthName  string        // "Julho 2026"
    Days       []CalendarDay // 35 or 42 cells (5-6 rows × 7 cols)
    PrevMonth  string        // "2026-06" for navigation
    NextMonth  string        // "2026-08" for navigation
}

// buildCalendarMonth produces the calendar data for a given month/year and patient appointments.
func buildCalendarMonth(year, month int, appointments []domain.Appointment) CalendarMonth {
    // 1. Determine first day of month's weekday offset
    // 2. Fill padding cells (Day=0) for days before the 1st
    // 3. Fill each day 1..lastDay, attaching appointments scheduled on that day
    // 4. Mark today if month/year matches current date
    // 5. Compute prev/next month strings
}
```

### 2. Patient Detail Page Handler (enhanced)

The existing `pagePsychPatientDetail` handler is updated to:
1. Build `CalendarMonth` data for the current month
2. Compute metrics (hours, payments) for the current month
3. Pass all data to the enhanced template

```go
type patientDetailData struct {
    PageData
    Patient      *domain.Patient
    Calendar     CalendarMonth
    Metrics      PatientMetrics
    SessionNotes []domain.SessionNote
    Documents    []domain.Document
    Contracts    []domain.Contract
    Payments     []domain.Payment
}

type PatientMetrics struct {
    PatientMinutes  int
    AnalysisMinutes int
    AdminMinutes    int
    TotalMinutes    int
    PendingCents    int64
    ReceivedCents   int64
}
```

### 3. Calendar htmx Fragment Endpoint

A new page-level handler that returns only the calendar grid HTML (not the full page), used by htmx for month navigation.

```go
// GET /psych/patients/:id/calendar?year=2026&month=7
func (a *App) pagePsychPatientCalendar(c *gin.Context) {
    // Parse year/month from query params (default: current month)
    // Fetch appointments for the patient in that month
    // Build CalendarMonth
    // Render only the calendar partial template (not full page)
}
```

Registered in `registerPsychPages`:
```go
r.GET("/patients/:id/calendar", a.pagePsychPatientCalendar)
```

### 4. Test Data Cleanup Endpoint

New handler in `handlers_dev.go`:

```go
// DELETE /api/dev/test-data
func (a *App) devDeleteTestData(c *gin.Context) {
    if !a.requireDevSecret(c) {
        return
    }
    db, err := a.dbForTenant(a.Config.DefaultTenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    counts, err := db.DeleteTestData()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, counts)
}
```

### 5. Storage: DeleteTestData

```go
// DeleteTestData removes all patients whose name starts with "TEST " OR email ends with "@test.com",
// plus all associated data (appointments, session_notes, documents, contracts, payments).
// Returns a map of entity type → count deleted.
func (db *DB) DeleteTestData() (map[string]int, error) {
    // 1. Find patient IDs matching criteria
    // 2. Delete from child tables using IN (patient_ids...)
    // 3. Delete patients
    // 4. Return counts
}
```

### 6. Storage: Patient-Specific Metrics Queries

```go
// SessionNoteHoursForPatientMonth returns total minutes by category for a patient in a month.
func (db *DB) SessionNoteHoursForPatientMonth(patientID string, month, year int) (patientMin, analysisMin, adminMin int, err error)

// PatientPaymentSummary returns pending and received totals for a patient (all time or current month).
func (db *DB) PatientPaymentSummary(patientID string) (pendingCents, receivedCents int64, err error)
```

### 7. Test Seed Script

`scripts/seed-test-data.sh` — a bash script that:
1. Creates 3 test patients (TEST Ana Silva, TEST Bruno Costa, TEST Carla Lima)
2. Creates appointments for each across the current and previous month
3. Uses `X-Dev-Auth: dev-local` header
4. Prints summary on completion
5. Continues on individual failures

---

## Interfaces

### htmx Interactions

| Trigger | htmx Attribute | Target | Response |
|---------|---------------|--------|----------|
| Click prev month | `hx-get="/psych/patients/:id/calendar?year=Y&month=M"` | `#calendar-container` | Calendar grid HTML fragment |
| Click next month | `hx-get="/psych/patients/:id/calendar?year=Y&month=M"` | `#calendar-container` | Calendar grid HTML fragment |
| Click appointment dot | Alpine.js `@click` → opens modal with appointment data | Modal overlay | N/A (client-side) |
| Click empty day | Alpine.js `@click` → opens create form with date | Modal overlay | N/A (client-side) |
| Submit edit modal | `hx-patch="/api/psych/appointments/:id/notes"` | Toast notification + calendar refresh | JSON → htmx `hx-trigger="calendarRefresh"` |
| Submit create form | `hx-post="/api/psych/appointments"` | Toast notification + calendar refresh | JSON → htmx `hx-trigger="calendarRefresh"` |

### Alpine.js State Model

```javascript
// Patient detail page root state
{
  tab: 'calendar',            // Active tab
  modal: null,                // null | 'edit' | 'create'
  selectedDate: '',           // Pre-fill date for create
  selectedAppointment: null,  // Appointment object for edit
  error: '',                  // Error message in modal
  loading: false              // Loading state for API calls
}
```

### API Contracts (existing, used by frontend)

**GET /api/psych/appointments?from=RFC3339&to=RFC3339&patient_id=UUID**
```json
[
  {
    "id": "uuid",
    "patient_id": "uuid",
    "patient_name": "Name",
    "type": "in_person|online",
    "status": "scheduled|completed|cancelled|rescheduled",
    "scheduled_at": "2026-07-22T14:00:00Z",
    "duration_minutes": 50,
    "notes": "",
    "created_at": "...",
    "updated_at": "..."
  }
]
```

**POST /api/psych/appointments**
```json
// Request
{ "patient_id": "uuid", "type": "in_person", "scheduled_at": "2026-07-22T14:00:00Z", "duration_minutes": 50, "notes": "" }
// Response: 201 with created appointment object
```

**DELETE /api/dev/test-data** (new)
```json
// Response: 200
{
  "patients": 3,
  "appointments": 12,
  "session_notes": 5,
  "documents": 0,
  "contracts": 0,
  "payments": 8
}
```

---

## Data Models

No new database tables. All data uses existing tables: `patients`, `appointments`, `session_notes`, `documents`, `contracts`, `payments`.

### New Go Structs (view models for templates)

```go
// CalendarDay — see Components section above
// CalendarMonth — see Components section above
// PatientMetrics — see Components section above
```

These are not persisted; they are computed per-request from existing data.

---

## Template Structure

```
internal/web/templates/
├── psych/
│   ├── patient_detail.html          (full page, enhanced)
│   └── partials/
│       └── calendar_grid.html       (htmx fragment for month navigation)
```

The `patient_detail.html` template defines the full page with:
- Patient header card
- Tab navigation (Alpine.js)
- Calendar tab (default, includes `calendar_grid` partial inline + metrics panel)
- Evoluções tab (session notes table/cards)
- Documentos tab
- Contratos tab
- Financeiro tab
- Modal overlays (edit appointment, create appointment)

The `calendar_grid.html` partial is used both:
1. Inline in the full page render (initial load)
2. As a standalone response for htmx calendar navigation requests

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Patient not found | HTTP 404, plain text "Paciente não encontrado" |
| API error in modal submit | Display error in modal without closing; Alpine.js `error` state |
| Calendar fragment load failure | htmx displays error in `#calendar-container` via `hx-swap="innerHTML"` |
| Empty data (no appointments, no notes) | Render zero-state with the same layout (no crash, no error) |
| Invalid month/year params | Default to current month |
| DeleteTestData on non-dev mode | Route not registered; returns 404 |
| DeleteTestData without X-Dev-Auth | Returns 401 |

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Calendar month grid correctness

*For any* valid year (2000–2100) and month (1–12), the `buildCalendarMonth` function SHALL produce a `CalendarMonth` where the number of `CalendarDay` cells with `Day > 0` equals the actual number of days in that month, the first non-padding cell starts on the correct weekday, and all cells with `Day > 0` are sequential from 1 to last day.

**Validates: Requirements 1.1**

### Property 2: Appointment-to-day grouping accuracy

*For any* set of appointments within a month and a given patient, the `buildCalendarMonth` function SHALL place each appointment into the `CalendarDay` whose date matches the appointment's `scheduled_at` date (UTC), and the sum of all appointments across all days SHALL equal the total number of input appointments.

**Validates: Requirements 1.2, 1.4**

### Property 3: Month navigation offset correctness

*For any* `CalendarMonth` with year Y and month M, the `PrevMonth` field SHALL reference (Y, M-1) — wrapping December→January with year decrement — and the `NextMonth` field SHALL reference (Y, M+1) — wrapping January→December with year increment. The computed `from`/`to` date range for the referenced month SHALL span exactly from the 1st at 00:00:00 UTC to the 1st of the following month at 00:00:00 UTC.

**Validates: Requirements 2.2, 2.3**

### Property 4: Patient session note hours aggregation

*For any* set of session notes belonging to a patient in a given month, the `PatientMetrics.TotalMinutes` SHALL equal the sum of `DurationPatientMin + DurationAnalysisMin + DurationAdminMin` across all notes, and each sub-field SHALL equal the sum of its respective column. When the set is empty, all values SHALL be zero.

**Validates: Requirements 5.1, 5.4**

### Property 5: Patient payment aggregation by status

*For any* set of payments belonging to a patient, `PatientMetrics.PendingCents` SHALL equal the sum of `AmountCents` where `Status == "pending"`, and `PatientMetrics.ReceivedCents` SHALL equal the sum of `AmountCents` where `Status == "received"`. When the set is empty, both SHALL be zero.

**Validates: Requirements 5.2, 5.4**

### Property 6: Test cleanup removes exactly TEST-prefixed patients and their data

*For any* database state containing a mix of patients where some have names starting with "TEST " or emails ending with "@test.com" and others do not, calling `DeleteTestData` SHALL remove all matching patients and their associated records (appointments, session_notes, documents, contracts, payments), SHALL leave all non-matching patients and their data intact, and SHALL return counts matching the actual number of deletions per entity type.

**Validates: Requirements 7.1, 7.2, 8.4**

### Property 7: Appointment listing respects date range and patient filter

*For any* set of appointments in the database and any query with `from`, `to`, and `patient_id` parameters, the returned list SHALL contain only appointments where `scheduled_at >= from AND scheduled_at < to AND patient_id = patient_id`, and SHALL contain ALL such appointments (no omissions).

**Validates: Requirements 8.3**

### Property 8: Patient and appointment creation round trip

*For any* valid `RegisterPatientInput` (non-empty name and unique email), creating a patient via POST SHALL return an object with a non-empty UUID `id` and matching `name`/`email`. Subsequently, *for any* valid appointment input referencing that patient's `id`, creating an appointment SHALL return an object with a non-empty `id`, matching `patient_id`, and `scheduled_at` equal to the input.

**Validates: Requirements 8.1, 8.2**
