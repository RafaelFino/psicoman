CREATE TABLE IF NOT EXISTS staff_users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS patients (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    phone TEXT DEFAULT '',
    birth_date TEXT,
    google_sub TEXT DEFAULT '',
    anamnesis TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS appointments (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL REFERENCES patients(id),
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    scheduled_at TEXT NOT NULL,
    duration_minutes INTEGER NOT NULL DEFAULT 50,
    google_event_id TEXT DEFAULT '',
    meet_link TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    report_html TEXT DEFAULT '',
    cancellation_reason TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_appointments_patient ON appointments(patient_id);
CREATE INDEX IF NOT EXISTS idx_appointments_scheduled ON appointments(scheduled_at);

CREATE TABLE IF NOT EXISTS scheduling_rules (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    min_hours_to_cancel INTEGER NOT NULL DEFAULT 24,
    min_hours_to_reschedule INTEGER NOT NULL DEFAULT 24,
    max_reschedules_per_month INTEGER NOT NULL DEFAULT 2,
    allow_patient_cancel INTEGER NOT NULL DEFAULT 1,
    allow_patient_reschedule INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO scheduling_rules (id) VALUES (1);

CREATE TABLE IF NOT EXISTS payments (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL REFERENCES patients(id),
    appointment_id TEXT DEFAULT '',
    amount_cents INTEGER NOT NULL,
    status TEXT NOT NULL,
    due_date TEXT NOT NULL,
    received_at TEXT,
    invoice_number TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_payments_patient ON payments(patient_id);

CREATE TABLE IF NOT EXISTS costs (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    month INTEGER NOT NULL,
    year INTEGER NOT NULL,
    category TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL REFERENCES patients(id),
    appointment_id TEXT DEFAULT '',
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    path TEXT NOT NULL,
    uploaded_by TEXT NOT NULL,
    doc_type TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_patient ON documents(patient_id);

CREATE TABLE IF NOT EXISTS google_tokens (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expiry TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
