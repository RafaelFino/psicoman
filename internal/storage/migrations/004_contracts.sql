CREATE TABLE IF NOT EXISTS contract_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    content_html TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS contracts (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL REFERENCES patients(id),
    template_id TEXT NOT NULL REFERENCES contract_templates(id),
    status TEXT NOT NULL DEFAULT 'pending',
    generated_html TEXT NOT NULL,
    signed_at TEXT,
    signature_ip TEXT DEFAULT '',
    signature_user_agent TEXT DEFAULT '',
    pdf_path TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_contracts_patient ON contracts(patient_id);
