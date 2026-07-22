CREATE TABLE IF NOT EXISTS supervisors (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT DEFAULT '',
    specialty TEXT DEFAULT '',
    crp TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS supervision_sessions (
    id TEXT PRIMARY KEY,
    supervisor_id TEXT NOT NULL REFERENCES supervisors(id),
    scheduled_at TEXT NOT NULL,
    duration_minutes INTEGER DEFAULT 60,
    notes_html TEXT DEFAULT '',
    topics TEXT DEFAULT '',
    cost_cents INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'scheduled',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_supervision_scheduled ON supervision_sessions(scheduled_at);
