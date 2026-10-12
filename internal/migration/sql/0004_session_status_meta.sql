-- Migration 0004 — metadados de status da sessão (patient-timeline R6.3).
-- A sessão passa a suportar o estado `confirmada` e metadados de cancelamento
-- (quem cancelou, quando, por quê) e de confirmação. Ver docs/architecture.md
-- §3/§4.1 e a spec .kiro/specs/patient-timeline (D6.2).
--
-- SQLite NÃO altera o CHECK de uma coluna por ALTER TABLE. Como o schema
-- inicial define `status ... CHECK (status IN (...))` sem `confirmada`, aceitar
-- o novo estado exige **recriar a tabela** (table rebuild): criar session_new
-- com o CHECK atualizado e as 4 colunas novas (nulas nos registros legados),
-- copiar os dados, trocar e recriar os índices.
--
-- Aditiva e idempotente no boot (o framework de migrations aplica uma única vez,
-- dentro de uma transação). O rebuild roda dentro dessa transação; as FKs são
-- definidas na própria session_new, então não é preciso mexer em PRAGMA.

CREATE TABLE session_new (
    id             TEXT PRIMARY KEY,
    patient_id     TEXT NOT NULL,
    location_id    TEXT,
    request_id     TEXT,
    modality       TEXT NOT NULL CHECK (modality IN ('presencial','online')),
    starts_at      TEXT NOT NULL,
    ends_at        TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'solicitada'
        CHECK (status IN ('solicitada','agendada','confirmada','realizada','cancelada','falta')),
    bill           INTEGER NOT NULL DEFAULT 0,
    consider_cost  INTEGER NOT NULL DEFAULT 0,
    google_event_id TEXT,
    meet_url       TEXT,
    confirmed_at   TEXT,
    canceled_by    TEXT,
    cancel_reason  TEXT,
    canceled_at    TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    deleted_at     TEXT,
    FOREIGN KEY (patient_id) REFERENCES patient(id) ON DELETE CASCADE,
    FOREIGN KEY (location_id) REFERENCES location(id),
    FOREIGN KEY (request_id) REFERENCES appointment_request(id)
);

INSERT INTO session_new
    (id, patient_id, location_id, request_id, modality, starts_at, ends_at,
     status, bill, consider_cost, google_event_id, meet_url,
     confirmed_at, canceled_by, cancel_reason, canceled_at,
     created_at, updated_at, deleted_at)
SELECT
     id, patient_id, location_id, request_id, modality, starts_at, ends_at,
     status, bill, consider_cost, google_event_id, meet_url,
     NULL, NULL, NULL, NULL,
     created_at, updated_at, deleted_at
FROM session;

DROP TABLE session;
ALTER TABLE session_new RENAME TO session;

CREATE INDEX ix_session_patient ON session(patient_id);
CREATE INDEX ix_session_status ON session(status);
CREATE INDEX ix_session_starts ON session(starts_at);
