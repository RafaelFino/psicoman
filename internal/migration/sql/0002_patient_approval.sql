-- Migration 0002 — gate de aprovação de paciente (mvp-audit1 R1).
-- Todo paciente que se cadastra pelo portal nasce `pendente` e só acessa os
-- recursos após aprovação do terapeuta. Ver docs/architecture.md §3 e a spec
-- .kiro/specs/mvp-audit1.
--
-- Aditiva e idempotente no boot (o framework de migrations só aplica uma vez):
--  1. Adiciona a coluna com DEFAULT 'pendente' (para novos registros do portal).
--  2. Marca os registros pré-existentes como 'aprovado' — foram cadastrados
--     pelo terapeuta, que é a autoridade de aprovação (R1.1).

ALTER TABLE patient ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'pendente';
UPDATE patient SET approval_status = 'aprovado';
