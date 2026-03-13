CREATE TABLE reminders
(
    id              BIGSERIAL,
    user_id         BIGINT      NOT NULL,
    medicine_id     BIGINT      NOT NULL,
    schedule_id     BIGINT      NOT NULL,
    scheduled_at    TIMESTAMPTZ NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'pending',
    retry_count     SMALLINT    NOT NULL DEFAULT 0,
    postpone_count  SMALLINT    NOT NULL DEFAULT 0,
    idempotency_key UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Primary key must include the partition key.
    PRIMARY KEY (id, scheduled_at)
) PARTITION BY RANGE (scheduled_at);

-- ── Partitions ─────────────────────────────────────────────────────────────
-- 2026
CREATE TABLE reminders_2026_03 PARTITION OF reminders FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE reminders_2026_04 PARTITION OF reminders FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE reminders_2026_05 PARTITION OF reminders FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE reminders_2026_06 PARTITION OF reminders FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE reminders_2026_07 PARTITION OF reminders FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE reminders_2026_08 PARTITION OF reminders FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE reminders_2026_09 PARTITION OF reminders FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE reminders_2026_10 PARTITION OF reminders FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE reminders_2026_11 PARTITION OF reminders FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE reminders_2026_12 PARTITION OF reminders FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

-- 2027
CREATE TABLE reminders_2027_01 PARTITION OF reminders FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');
CREATE TABLE reminders_2027_02 PARTITION OF reminders FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');
CREATE TABLE reminders_2027_03 PARTITION OF reminders FOR VALUES FROM ('2027-03-01') TO ('2027-04-01');

-- User's history sorted newest-first (most common read query).
CREATE INDEX idx_reminders_user_time
    ON reminders (user_id, scheduled_at DESC);

-- Idempotency: prevents duplicate reminders from the scheduler.
CREATE UNIQUE INDEX uidx_reminders_idempotency
    ON reminders (idempotency_key);

-- Scheduler re-generates reminders per schedule; lets it check existing ones.
CREATE INDEX idx_reminders_schedule
    ON reminders (schedule_id);

-- Analytics: confirmed rate over a time window.
CREATE INDEX idx_reminders_confirmed_time
    ON reminders (scheduled_at)
    WHERE status = 'confirmed';

-- Combined: "how many skips did user X have?"
CREATE INDEX idx_reminders_user_status
    ON reminders (user_id, status);