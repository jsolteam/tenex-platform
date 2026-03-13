CREATE TABLE intakes
(
    id             BIGSERIAL PRIMARY KEY,
    reminder_id    BIGINT      NOT NULL, -- logical FK; physical FK omitted due to partitioning
    status         VARCHAR(32) NOT NULL, -- confirmed | skipped
    proof_media_id BIGINT      REFERENCES media (id) ON DELETE SET NULL,
    reason         TEXT,                 -- required when status = skipped
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Lookup: "all intakes for reminder X"
CREATE INDEX idx_intakes_reminder_id ON intakes (reminder_id);

-- Analytics join: "all confirmed intakes with proof"
CREATE INDEX idx_intakes_status ON intakes (status, created_at DESC);