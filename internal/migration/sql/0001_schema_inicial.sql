-- Migration 0001 — schema inicial do Psicoman.
-- Convenções: PK id TEXT (ULID); timestamps TEXT ISO-8601 (America/Sao_Paulo);
-- dinheiro em INTEGER (centavos, BRL); soft-delete via deleted_at onde aplicável.
-- Ver docs/architecture.md §3.

-- Origem / canal de aquisição (Doctoralia, indicação, etc.).
CREATE TABLE origin (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

-- Paciente. email UNIQUE; cpf UNIQUE quando não nulo (índice parcial abaixo).
CREATE TABLE patient (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    phone      TEXT NOT NULL,
    email      TEXT NOT NULL,
    cpf        TEXT,
    origin_id  TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    FOREIGN KEY (origin_id) REFERENCES origin(id)
);
CREATE UNIQUE INDEX ux_patient_email ON patient(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ux_patient_cpf ON patient(cpf) WHERE cpf IS NOT NULL AND deleted_at IS NULL;

-- Local de atendimento.
CREATE TABLE location (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    address     TEXT,
    modality    TEXT NOT NULL CHECK (modality IN ('presencial','online')),
    cost_amount INTEGER NOT NULL DEFAULT 0,           -- centavos
    cost_period TEXT NOT NULL DEFAULT 'por_sessao'
        CHECK (cost_period IN ('por_sessao','diario','mensal','anual')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    deleted_at  TEXT
);

-- Disponibilidade de agenda de um local (janelas de atendimento).
CREATE TABLE availability (
    id          TEXT PRIMARY KEY,
    location_id TEXT NOT NULL,
    weekday     INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6), -- 0=domingo
    start_time  TEXT NOT NULL,   -- "HH:MM"
    end_time    TEXT NOT NULL,   -- "HH:MM"
    capacity    INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    deleted_at  TEXT,
    FOREIGN KEY (location_id) REFERENCES location(id) ON DELETE CASCADE
);
CREATE INDEX ix_availability_location ON availability(location_id);

-- Plano/acordo de pagamento por paciente.
CREATE TABLE plan (
    id          TEXT PRIMARY KEY,
    patient_id  TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN (
                    'pagamento_por_consulta','pagamento_por_mes',
                    'plano_fechado_mensal','plano_fechado_trimestral',
                    'atendimento_social')),
    amount      INTEGER NOT NULL DEFAULT 0,  -- centavos (para planos fixos)
    starts_at   TEXT NOT NULL,
    ends_at     TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    deleted_at  TEXT,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE
);
CREATE INDEX ix_plan_patient ON plan(patient_id);

-- Pedido de agendamento (registro interno do portal; não toca o Google).
CREATE TABLE appointment_request (
    id           TEXT PRIMARY KEY,
    patient_id   TEXT NOT NULL,
    location_id  TEXT,
    slot_start   TEXT NOT NULL,
    slot_end     TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pendente'
        CHECK (status IN ('pendente','confirmado','recusado')),
    note         TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (location_id) REFERENCES location(id)
);
CREATE INDEX ix_apptreq_status ON appointment_request(status);
CREATE INDEX ix_apptreq_patient ON appointment_request(patient_id);

-- Sessão.
CREATE TABLE session (
    id             TEXT PRIMARY KEY,
    patient_id     TEXT NOT NULL,
    location_id    TEXT,
    request_id     TEXT,
    modality       TEXT NOT NULL CHECK (modality IN ('presencial','online')),
    starts_at      TEXT NOT NULL,
    ends_at        TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'solicitada'
        CHECK (status IN ('solicitada','agendada','realizada','cancelada','falta')),
    bill           INTEGER NOT NULL DEFAULT 0,  -- boolean 0/1: haverá cobrança
    consider_cost  INTEGER NOT NULL DEFAULT 0,  -- boolean 0/1: considerar custos
    google_event_id TEXT,
    meet_url       TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    deleted_at     TEXT,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (location_id) REFERENCES location(id),
    FOREIGN KEY (request_id) REFERENCES appointment_request(id)
);
CREATE INDEX ix_session_patient ON session(patient_id);
CREATE INDEX ix_session_status ON session(status);
CREATE INDEX ix_session_starts ON session(starts_at);

-- Débito (valor a receber). idempotency_key UNIQUE garante geração idempotente.
CREATE TABLE debt (
    id              TEXT PRIMARY KEY,
    patient_id      TEXT NOT NULL,
    session_id      TEXT,             -- nulo para débitos de plano fechado
    plan_id         TEXT,
    billing_period  TEXT,             -- 'YYYY-MM' ou 'YYYY-Q' quando aplicável
    amount          INTEGER NOT NULL, -- centavos
    due_date        TEXT,
    status          TEXT NOT NULL DEFAULT 'aberto'
        CHECK (status IN ('aberto','pago','parcial')),
    idempotency_key TEXT NOT NULL,
    pdf_ged_file_id TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    deleted_at      TEXT,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES session(id),
    FOREIGN KEY (plan_id) REFERENCES plan(id)
);
CREATE UNIQUE INDEX ux_debt_idempotency ON debt(idempotency_key);
CREATE INDEX ix_debt_patient ON debt(patient_id);
CREATE INDEX ix_debt_status ON debt(status);

-- Pagamento (quita débito, total ou parcial).
CREATE TABLE payment (
    id         TEXT PRIMARY KEY,
    debt_id    TEXT NOT NULL,
    amount     INTEGER NOT NULL,  -- centavos
    paid_at    TEXT NOT NULL,
    method     TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (debt_id) REFERENCES debt(id) ON DELETE CASCADE
);
CREATE INDEX ix_payment_debt ON payment(debt_id);

-- Categoria e item de custo.
CREATE TABLE cost_category (
    id         TEXT PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('local','crp','infra','plataforma')),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE cost_item (
    id          TEXT PRIMARY KEY,
    category_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    amount      INTEGER NOT NULL,  -- centavos
    period      TEXT NOT NULL CHECK (period IN ('por_sessao','diario','mensal','anual')),
    origin_id   TEXT,              -- para custos de plataforma (ROI)
    location_id TEXT,              -- para custos de local
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    deleted_at  TEXT,
    FOREIGN KEY (category_id) REFERENCES cost_category(id) ON DELETE CASCADE,
    FOREIGN KEY (origin_id) REFERENCES origin(id),
    FOREIGN KEY (location_id) REFERENCES location(id)
);
CREATE INDEX ix_cost_item_category ON cost_item(category_id);
CREATE INDEX ix_cost_item_origin ON cost_item(origin_id);

-- Custo atribuído a uma sessão (direto ou rateio), com snapshot auditável.
CREATE TABLE session_cost (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL,
    amount       INTEGER NOT NULL,  -- centavos
    method       TEXT NOT NULL CHECK (method IN ('direto','rateio')),
    base_snapshot TEXT,             -- JSON com a base do rateio (auditoria)
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES session(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX ux_session_cost_session ON session_cost(session_id);

-- Anamnese (uma por paciente).
CREATE TABLE anamnesis (
    id         TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL,
    content    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX ux_anamnesis_patient ON anamnesis(patient_id);

-- Nota (de sessão quando session_id não nulo; livre caso contrário).
CREATE TABLE note (
    id         TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL,
    session_id TEXT,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES session(id) ON DELETE SET NULL
);
CREATE INDEX ix_note_patient ON note(patient_id, created_at);
CREATE INDEX ix_note_session ON note(session_id);

-- Template Markdown e registro de envio.
CREATE TABLE template (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    body_md    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE TABLE template_send (
    id           TEXT PRIMARY KEY,
    template_id  TEXT NOT NULL,
    patient_id   TEXT NOT NULL,
    rendered_html TEXT NOT NULL,
    sent_at      TEXT NOT NULL,
    FOREIGN KEY (template_id) REFERENCES template(id),
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE
);
CREATE INDEX ix_template_send_patient ON template_send(patient_id);

-- Arquivo no GED (segregado por paciente; vínculos opcionais).
CREATE TABLE ged_file (
    id          TEXT PRIMARY KEY,
    patient_id  TEXT,             -- nulo p/ arquivos do perfil do terapeuta
    session_id  TEXT,
    debt_id     TEXT,
    payment_id  TEXT,
    rel_path    TEXT NOT NULL,    -- caminho relativo dentro do GED
    mime        TEXT,
    size        INTEGER NOT NULL DEFAULT 0,
    sha256      TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES session(id) ON DELETE SET NULL,
    FOREIGN KEY (debt_id) REFERENCES debt(id) ON DELETE SET NULL,
    FOREIGN KEY (payment_id) REFERENCES payment(id) ON DELETE SET NULL
);
CREATE INDEX ix_ged_patient ON ged_file(patient_id);
-- dedup por hash dentro do escopo do paciente
CREATE UNIQUE INDEX ux_ged_patient_hash ON ged_file(patient_id, sha256);

-- Token OAuth do Google (refresh token cifrado).
CREATE TABLE google_token (
    id                    TEXT PRIMARY KEY,
    refresh_token_enc     TEXT NOT NULL,   -- cifrado (AES-GCM)
    scopes                TEXT NOT NULL,
    access_token_expiry   TEXT,
    reauth_required        INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT NOT NULL,
    updated_at            TEXT NOT NULL
);

-- Perfil do terapeuta (um por instância).
CREATE TABLE therapist_profile (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    crp          TEXT,
    email        TEXT,
    contacts     TEXT,             -- JSON (telefone e outros)
    bio          TEXT,
    photo_ged_file_id TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (photo_ged_file_id) REFERENCES ged_file(id)
);

-- Associação perfil ↔ local (locais onde o terapeuta atende).
CREATE TABLE therapist_location (
    profile_id  TEXT NOT NULL,
    location_id TEXT NOT NULL,
    PRIMARY KEY (profile_id, location_id),
    FOREIGN KEY (profile_id) REFERENCES therapist_profile(id) ON DELETE CASCADE,
    FOREIGN KEY (location_id) REFERENCES location(id) ON DELETE CASCADE
);

-- Links de plataformas do perfil (Doctoralia, Zenklub, etc.).
CREATE TABLE therapist_platform_link (
    id         TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    label      TEXT NOT NULL,
    url        TEXT NOT NULL,
    origin_id  TEXT,             -- reaproveita origem quando é canal de aquisição
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (profile_id) REFERENCES therapist_profile(id) ON DELETE CASCADE,
    FOREIGN KEY (origin_id) REFERENCES origin(id)
);
CREATE INDEX ix_platform_link_profile ON therapist_platform_link(profile_id);

-- Audit log de operações sensíveis.
CREATE TABLE audit_log (
    id         TEXT PRIMARY KEY,
    actor      TEXT NOT NULL,     -- email
    action     TEXT NOT NULL,
    entity     TEXT,
    entity_id  TEXT,
    metadata   TEXT,              -- JSON (nunca conteúdo clínico)
    created_at TEXT NOT NULL
);
CREATE INDEX ix_audit_actor ON audit_log(actor, created_at);
CREATE INDEX ix_audit_entity ON audit_log(entity, entity_id);
