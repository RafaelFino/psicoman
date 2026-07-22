CREATE TABLE IF NOT EXISTS session_notes (
    id TEXT PRIMARY KEY,
    appointment_id TEXT NOT NULL REFERENCES appointments(id),
    patient_id TEXT NOT NULL REFERENCES patients(id),
    content_html TEXT DEFAULT '',
    private_notes TEXT DEFAULT '',
    duration_patient_min INTEGER DEFAULT 0,
    duration_analysis_min INTEGER DEFAULT 0,
    duration_admin_min INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_notes_patient ON session_notes(patient_id);
CREATE INDEX IF NOT EXISTS idx_session_notes_appointment ON session_notes(appointment_id);
