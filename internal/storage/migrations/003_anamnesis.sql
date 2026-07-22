CREATE TABLE IF NOT EXISTS anamnesis_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    target_age_group TEXT DEFAULT 'adult',
    fields_json TEXT NOT NULL DEFAULT '[]',
    is_active INTEGER DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS anamnesis_responses (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL REFERENCES patients(id),
    template_id TEXT NOT NULL REFERENCES anamnesis_templates(id),
    responses_json TEXT NOT NULL DEFAULT '{}',
    completed_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_anamnesis_responses_patient ON anamnesis_responses(patient_id);
