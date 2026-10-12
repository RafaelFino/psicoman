-- Migration 0003 — dia de vencimento dos planos fechados (patient-timeline R1.5).
-- Os planos fechados (mensal/trimestral) passam a cobrar por ciclo a partir de
-- um dia de vencimento (1–28), em vez de "+7 dias" arbitrário. Ver
-- docs/architecture.md §4.1.1 e a spec .kiro/specs/patient-timeline.
--
-- Aditiva e idempotente no boot (o framework de migrations só aplica uma vez):
--  1. Adiciona a coluna com DEFAULT 1 (válido para todos; irrelevante nos
--     tipos que não geram ciclo).
--  2. Dá um padrão coerente (dia 5) aos planos fechados pré-existentes; demais
--     tipos ficam com 1 (nunca geram ciclo, então o valor é indiferente).

ALTER TABLE plan ADD COLUMN due_day INTEGER NOT NULL DEFAULT 1;
UPDATE plan SET due_day = 5 WHERE type IN ('plano_fechado_mensal','plano_fechado_trimestral');
