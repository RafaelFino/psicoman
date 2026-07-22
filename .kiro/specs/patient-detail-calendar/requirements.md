# Requirements Document

## Introduction

Evolução da página de detalhe do paciente (`/psych/patients/:id`) para incluir um calendário mensal interativo de consultas, navegação por abas completa (Calendário, Evoluções, Documentos, Contratos, Financeiro), métricas visuais com CSS/SVG, e infraestrutura de dados de teste (scripts bash + endpoint de limpeza + testes automatizados Go).

## Glossary

- **Patient_Detail_Page**: Página HTML renderizada pelo Go em `/psych/patients/:id` usando Go html/template + htmx + Alpine.js, exibindo informações detalhadas de um paciente específico.
- **Monthly_Calendar**: Componente visual de calendário mensal renderizado em HTML/CSS dentro da Patient_Detail_Page, exibindo os dias do mês com indicadores de consultas agendadas.
- **Appointment_Edit_Modal**: Modal/dialog HTML ativado ao clicar em uma consulta existente no Monthly_Calendar, permitindo editar campos da consulta via API.
- **Appointment_Create_Form**: Formulário para criação de nova consulta, pré-preenchido com a data selecionada ao clicar em um dia vazio do Monthly_Calendar.
- **Tab_Navigation**: Navegação por abas Alpine.js na Patient_Detail_Page com as seções: Calendário, Evoluções, Documentos, Contratos, Financeiro.
- **Test_Seed_Script**: Script bash usando curl que cria dados de teste no sistema (nomes com prefixo "TEST", emails @test.com).
- **Test_Cleanup_Endpoint**: Endpoint `DELETE /api/dev/test-data` que remove todos os dados de teste identificáveis pelo prefixo/domínio.
- **Metrics_Panel**: Seção visual da aba Calendário exibindo horas trabalhadas e valores de pagamentos usando CSS/SVG charts.
- **Psych_API**: Conjunto de endpoints JSON em `/api/psych/*` autenticados via header X-Dev-Auth (dev) ou Pangolin headers (produção).

## Requirements

### Requirement 1: Monthly Calendar Display

**User Story:** As a psychologist, I want to see a monthly calendar on the patient detail page, so that I can quickly visualize the appointment schedule for a specific patient.

#### Acceptance Criteria

1. WHEN the psychologist navigates to the "Calendário" tab on the Patient_Detail_Page, THE Monthly_Calendar SHALL render a grid displaying all days of the current month with the month name and year as header.
2. WHEN a day in the Monthly_Calendar has one or more scheduled appointments for the patient, THE Monthly_Calendar SHALL display a visual indicator (colored dot or badge) showing the number of appointments on that day.
3. THE Monthly_Calendar SHALL highlight the current day with a distinct visual style differentiating the current day from other days.
4. WHEN the patient has no appointments in the displayed month, THE Monthly_Calendar SHALL render the full month grid with no appointment indicators and no error messages.

### Requirement 2: Calendar Navigation

**User Story:** As a psychologist, I want to navigate between months in the calendar, so that I can review past and future appointments for the patient.

#### Acceptance Criteria

1. THE Monthly_Calendar SHALL display "previous month" and "next month" navigation buttons adjacent to the month/year header.
2. WHEN the psychologist clicks the "previous month" button, THE Monthly_Calendar SHALL load and display the appointments for the preceding month by calling the Psych_API with updated date range parameters.
3. WHEN the psychologist clicks the "next month" button, THE Monthly_Calendar SHALL load and display the appointments for the following month by calling the Psych_API with updated date range parameters.
4. WHILE the Monthly_Calendar is loading appointment data for a new month, THE Patient_Detail_Page SHALL display a loading indicator within the calendar area.

### Requirement 3: Appointment Interaction from Calendar

**User Story:** As a psychologist, I want to click appointments on the calendar to edit them and click empty days to create new appointments, so that I can manage the schedule directly from the patient view.

#### Acceptance Criteria

1. WHEN the psychologist clicks on an appointment indicator in the Monthly_Calendar, THE Patient_Detail_Page SHALL open the Appointment_Edit_Modal populated with the existing appointment data (date, time, type, duration, notes).
2. WHEN the psychologist submits the Appointment_Edit_Modal with valid changes, THE Patient_Detail_Page SHALL send the update to the Psych_API and refresh the Monthly_Calendar to reflect the changes.
3. WHEN the psychologist clicks on an empty day (a day with no appointments) in the Monthly_Calendar, THE Patient_Detail_Page SHALL open the Appointment_Create_Form with the date field pre-filled to the clicked date and the patient_id pre-filled.
4. WHEN the psychologist submits the Appointment_Create_Form with valid data, THE Patient_Detail_Page SHALL send a POST request to the Psych_API and refresh the Monthly_Calendar to show the newly created appointment.
5. IF the Psych_API returns an error during appointment creation or editing, THEN THE Patient_Detail_Page SHALL display the error message to the psychologist without closing the modal or form.

### Requirement 4: Tab Navigation

**User Story:** As a psychologist, I want a tabbed interface on the patient detail page, so that I can quickly switch between calendar, session notes, documents, contracts, and financial information.

#### Acceptance Criteria

1. THE Patient_Detail_Page SHALL display a Tab_Navigation component with five tabs: "Calendário", "Evoluções", "Documentos", "Contratos", "Financeiro".
2. WHEN the psychologist clicks a tab in the Tab_Navigation, THE Patient_Detail_Page SHALL display the content panel corresponding to the selected tab and hide all other tab panels.
3. THE Tab_Navigation SHALL visually indicate the currently active tab with a distinct style (e.g., highlighted border or background color).
4. WHEN the Patient_Detail_Page loads, THE Tab_Navigation SHALL default to the "Calendário" tab as the active tab.

### Requirement 5: Metrics Panel

**User Story:** As a psychologist, I want to see visual metrics (hours worked, payments) on the patient detail page, so that I can have a quick financial and workload overview for the patient.

#### Acceptance Criteria

1. THE Metrics_Panel SHALL display the total hours of session notes (patient minutes + analysis minutes + admin minutes) for the patient in the current month.
2. THE Metrics_Panel SHALL display the total pending payment amount and total received payment amount for the patient.
3. THE Metrics_Panel SHALL render hours and payments data using CSS-styled visual elements (progress bars or SVG mini-charts) providing at-a-glance comprehension.
4. WHEN the patient has no session notes or payments, THE Metrics_Panel SHALL display zero values with the same visual layout without rendering errors.

### Requirement 6: Test Data Seed Script

**User Story:** As a developer, I want a bash script that seeds test data with identifiable prefixes, so that I can quickly populate the system for manual and automated testing.

#### Acceptance Criteria

1. THE Test_Seed_Script SHALL create patients using names prefixed with "TEST" (e.g., "TEST Ana Silva") and email addresses with the domain @test.com.
2. THE Test_Seed_Script SHALL create appointments for the test patients across multiple days and time slots.
3. THE Test_Seed_Script SHALL authenticate all API calls using the X-Dev-Auth header with the configured dev secret.
4. THE Test_Seed_Script SHALL output a summary of created resources (patient count, appointment count) upon completion.
5. IF any API call in the Test_Seed_Script fails, THEN THE Test_Seed_Script SHALL print an error message identifying the failed operation and continue with the remaining operations.

### Requirement 7: Test Data Cleanup Endpoint

**User Story:** As a developer, I want an API endpoint that removes all test data, so that I can reset the system to a clean state after testing.

#### Acceptance Criteria

1. WHEN a DELETE request is received at `/api/dev/test-data` with a valid X-Dev-Auth header, THE Test_Cleanup_Endpoint SHALL delete all patients whose names start with "TEST" and all associated data (appointments, session notes, documents, contracts, payments).
2. WHEN the deletion completes, THE Test_Cleanup_Endpoint SHALL return a JSON response with counts of deleted records per entity type.
3. IF the X-Dev-Auth header is missing or invalid, THEN THE Test_Cleanup_Endpoint SHALL return HTTP 401 with an error message.
4. THE Test_Cleanup_Endpoint SHALL only be registered when the application runs in DEV_MODE.

### Requirement 8: Automated Tests

**User Story:** As a developer, I want Go httptest tests covering the patient detail calendar flows, so that regressions are caught automatically.

#### Acceptance Criteria

1. THE automated tests SHALL verify that creating a patient via `POST /api/psych/patients` returns HTTP 201 with a valid patient object containing an id field.
2. THE automated tests SHALL verify that creating an appointment for a patient via `POST /api/psych/appointments` returns HTTP 201 with the appointment linked to the correct patient_id.
3. THE automated tests SHALL verify that listing appointments with `from` and `to` query parameters and a `patient_id` filter returns only appointments matching the specified patient and date range.
4. THE automated tests SHALL verify that the `DELETE /api/dev/test-data` endpoint removes test-prefixed patients and their associated appointments.
5. THE automated tests SHALL use `httptest` with the full Gin router and an isolated temporary SQLite database per test.

### Requirement 9: Responsive and Accessible UI

**User Story:** As a psychologist, I want the patient detail page to be usable on different screen sizes and accessible via keyboard, so that I can work comfortably from any device.

#### Acceptance Criteria

1. THE Monthly_Calendar SHALL use CSS Grid or Flexbox layout that adapts to viewport widths from 375px to 1920px without horizontal scrolling.
2. THE Tab_Navigation SHALL be navigable via keyboard (Tab key to move between tabs, Enter/Space to activate).
3. THE Appointment_Edit_Modal SHALL be closeable via the Escape key and trap focus within the modal while open.
4. THE Patient_Detail_Page tables (Evoluções, Documentos, Contratos, Financeiro) SHALL display as responsive cards on viewports narrower than 768px.
