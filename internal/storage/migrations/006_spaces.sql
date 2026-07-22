CREATE TABLE IF NOT EXISTS therapy_spaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT DEFAULT '',
    type TEXT NOT NULL DEFAULT 'fixed',
    cost_cents_per_use INTEGER DEFAULT 0,
    cost_cents_monthly INTEGER DEFAULT 0,
    is_available INTEGER DEFAULT 1,
    notes TEXT DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS space_bookings (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL REFERENCES therapy_spaces(id),
    appointment_id TEXT REFERENCES appointments(id),
    booking_date TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_space_bookings_date ON space_bookings(booking_date);
CREATE INDEX IF NOT EXISTS idx_space_bookings_space ON space_bookings(space_id);
