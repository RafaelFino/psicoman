# Implementation Plan: Patient Detail Calendar

## Overview

Enhance the patient detail page (`/psych/patients/:id`) with a monthly calendar, tab navigation, metrics panel, and test infrastructure. Implementation proceeds bottom-up: storage queries → service/page logic → templates/CSS → htmx endpoint → dev cleanup endpoint → seed script → automated tests.

## Tasks

- [x] 1. Storage layer: new queries for metrics and test cleanup
  - [x] 1.1 Add `SessionNoteHoursForPatientMonth` query to `internal/storage/session_note.go`
    - New method `func (db *DB) SessionNoteHoursForPatientMonth(patientID string, month, year int) (patientMin, analysisMin, adminMin int, err error)`
    - SQL: `SELECT COALESCE(SUM(duration_patient_min),0), COALESCE(SUM(duration_analysis_min),0), COALESCE(SUM(duration_admin_min),0) FROM session_notes WHERE patient_id=? AND ...month/year filter`
    - Return zeroes if no rows
    - _Requirements: 5.1, 5.4_

  - [x] 1.2 Add `PatientPaymentSummary` query to `internal/storage/finance.go`
    - New method `func (db *DB) PatientPaymentSummary(patientID string) (pendingCents, receivedCents int64, err error)`
    - SQL: aggregate `amount_cents` grouping by `status` IN ('pending','received') for the given patient
    - Return zeroes when no rows exist
    - _Requirements: 5.2, 5.4_

  - [x] 1.3 Add `DeleteTestData` method to `internal/storage/patient.go`
    - New method `func (db *DB) DeleteTestData() (map[string]int, error)`
    - Find all patient IDs where `name LIKE 'TEST %' OR email LIKE '%@test.com'`
    - Delete from child tables (appointments, session_notes, documents, contracts, payments) using `patient_id IN (...)`
    - Delete the patients themselves
    - Return counts per entity type
    - Wrap in a transaction for atomicity
    - _Requirements: 7.1, 7.2_

- [x] 2. Calendar data builder (pure Go logic)
  - [x] 2.1 Implement `buildCalendarMonth` in `internal/web/pages_psych.go`
    - Define `CalendarDay` and `CalendarMonth` structs as per design
    - Compute weekday offset of month's first day (week starts Sunday)
    - Fill padding cells (Day=0), then day cells 1..lastDay
    - Attach appointments to their matching day (compare UTC date)
    - Mark today if year/month match current date
    - Compute PrevMonth/NextMonth strings (handle Dec→Jan, Jan→Dec wraps)
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [ ]* 2.2 Write property tests for `buildCalendarMonth` in `internal/web/pages_psych_test.go`
    - **Property 1: Calendar month grid correctness** — for any valid year/month, number of days with Day>0 equals actual days in month, cells are sequential 1..lastDay
    - **Property 2: Appointment-to-day grouping accuracy** — every appointment lands in the correct CalendarDay, total count matches input
    - **Property 3: Month navigation offset correctness** — PrevMonth/NextMonth wrap correctly
    - **Validates: Requirements 1.1, 1.2, 1.4, 2.2, 2.3**

- [x] 3. Enhance patient detail page handler and add metrics
  - [x] 3.1 Update `patientDetailData` struct and `pagePsychPatientDetail` handler in `internal/web/pages_psych.go`
    - Add `Calendar CalendarMonth` and `Metrics PatientMetrics` fields to `patientDetailData`
    - Define `PatientMetrics` struct (PatientMinutes, AnalysisMinutes, AdminMinutes, TotalMinutes, PendingCents, ReceivedCents)
    - In the handler: call `buildCalendarMonth` with current month appointments
    - Call `db.SessionNoteHoursForPatientMonth` and `db.PatientPaymentSummary` to populate metrics
    - _Requirements: 1.1, 5.1, 5.2, 5.3, 5.4_

  - [x] 3.2 Add htmx calendar fragment endpoint `pagePsychPatientCalendar` in `internal/web/pages_psych.go`
    - New handler: `GET /psych/patients/:id/calendar?year=2026&month=7`
    - Parse year/month from query params (default: current month)
    - Fetch appointments for patient in the requested month
    - Build `CalendarMonth`, render only the `calendar_grid.html` partial
    - Register route in `registerPsychPages`
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 4. Checkpoint — Ensure Go compiles and existing tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Templates and CSS
  - [x] 5.1 Create `internal/web/templates/psych/partials/calendar_grid.html`
    - Render the monthly calendar grid using CSS Grid (7 columns)
    - Show month/year header with prev/next buttons using `hx-get` to the calendar fragment endpoint
    - Render each CalendarDay: padding cells empty, day cells with number and appointment dots
    - Highlight today with `.calendar-today` class
    - Include `hx-indicator` for loading state during navigation
    - Appointment dots clickable via Alpine.js `@click` to open edit modal
    - Empty day cells clickable to open create form with date pre-filled
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.3, 2.4, 3.1, 3.3, 9.1_

  - [x] 5.2 Update `internal/web/templates/psych/patient_detail.html` — full page with tabs and modals
    - Add Alpine.js root state (`tab`, `modal`, `selectedDate`, `selectedAppointment`, `error`, `loading`)
    - Implement Tab_Navigation with 5 tabs: Calendário, Evoluções, Documentos, Contratos, Financeiro
    - Default active tab: Calendário
    - Keyboard accessible tabs (Tab/Enter/Space via `@keydown`)
    - Calendar tab content: include `calendar_grid` partial inline + Metrics_Panel
    - Metrics panel: CSS progress bars for hours, payment amounts display
    - Other tabs: render tables/cards for existing data (session notes, documents, contracts, payments)
    - Responsive tables → cards at <768px via CSS media query
    - Appointment edit modal (Alpine.js controlled, `hx-patch` submit, Escape to close, focus trap)
    - Appointment create form modal (Alpine.js, `hx-post`, pre-filled date)
    - Error display in modals without closing
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 9.1, 9.2, 9.3, 9.4_

  - [x] 5.3 Add calendar and tab CSS to `internal/web/static/css/styles.css`
    - `.calendar-grid` with CSS Grid 7 columns, responsive collapse
    - `.calendar-day`, `.calendar-today`, `.calendar-dot` styles
    - `.tab-nav`, `.tab-active` styles
    - `.metrics-panel`, `.progress-bar` styles
    - Responsive card layout for tables at `@media (max-width: 768px)`
    - Modal overlay and focus trap styles
    - _Requirements: 5.3, 9.1, 9.4_

- [x] 6. Dev cleanup endpoint and seed script
  - [x] 6.1 Add `DELETE /api/dev/test-data` handler to `internal/web/handlers_dev.go`
    - New method `devDeleteTestData` — check dev secret, call `db.DeleteTestData()`, return JSON counts
    - Register in `registerDevRoutes` (only active in DEV_MODE)
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [x] 6.2 Create `scripts/seed-test-data.sh`
    - Create 3 test patients: "TEST Ana Silva", "TEST Bruno Costa", "TEST Carla Lima" with @test.com emails
    - Create appointments for each across current and previous month (varied times)
    - Use `X-Dev-Auth: dev-local` header for all calls
    - Print summary of created resources on completion
    - Continue on individual failures (print error and proceed)
    - Make script executable (`chmod +x`)
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 7. Checkpoint — Full compile + manual validation
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Automated tests
  - [x] 8.1 Add test `TestPatientDetailCalendarAPI` to `internal/web/handlers_test.go`
    - Test patient creation returns 201 with id
    - Test appointment creation returns 201 linked to correct patient_id
    - Test appointment listing with from/to/patient_id filters returns correct subset
    - Uses `httptest` + full Gin router + temporary SQLite DB (via existing `testApp` helper)
    - _Requirements: 8.1, 8.2, 8.3, 8.5_

  - [x] 8.2 Add test `TestDevDeleteTestData` to `internal/web/handlers_test.go`
    - Create TEST-prefixed patients and non-test patients
    - Call `DELETE /api/dev/test-data` with valid dev header
    - Verify TEST patients removed, non-test patients remain
    - Verify response counts match actual deletions
    - Test 401 when header is missing
    - _Requirements: 8.4, 8.5, 7.3_

  - [ ]* 8.3 Write property tests for storage metrics queries in `internal/storage/storage_test.go`
    - **Property 4: Patient session note hours aggregation** — TotalMinutes equals sum of all category minutes
    - **Property 5: Patient payment aggregation by status** — PendingCents/ReceivedCents match sums filtered by status
    - **Validates: Requirements 5.1, 5.2, 5.4**

  - [ ]* 8.4 Write property test for `DeleteTestData` isolation in `internal/storage/storage_test.go`
    - **Property 6: Test cleanup removes exactly TEST-prefixed patients and their data** — non-test data untouched
    - **Validates: Requirements 7.1, 7.2**

- [x] 9. Final checkpoint — All tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- The project uses Go (Gin), Go html/template with embed, htmx, and Alpine.js — no new dependencies needed
- All JSON fields use snake_case per project conventions
- Templates are embedded via `go:embed` in `internal/web/templates.go`

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3"] },
    { "id": 1, "tasks": ["2.1", "6.1"] },
    { "id": 2, "tasks": ["2.2", "3.1"] },
    { "id": 3, "tasks": ["3.2", "5.3"] },
    { "id": 4, "tasks": ["5.1", "5.2"] },
    { "id": 5, "tasks": ["6.2"] },
    { "id": 6, "tasks": ["8.1", "8.2"] },
    { "id": 7, "tasks": ["8.3", "8.4"] }
  ]
}
```
