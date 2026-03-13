CREATE TABLE user_statistics
(
    user_id         BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    total_reminders INT         NOT NULL DEFAULT 0,
    confirmed       INT         NOT NULL DEFAULT 0,
    skipped         INT         NOT NULL DEFAULT 0,
    adherence_rate  FLOAT       NOT NULL DEFAULT 0, -- confirmed / total_reminders [0..1]
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE platform_config
(
    key        TEXT PRIMARY KEY,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed sensible defaults so DBProvider has data to return even before
-- any runtime overrides are applied.
INSERT INTO platform_config (key, value)
VALUES ('scheduler.max_retries', '3'),
       ('scheduler.retry_interval_seconds', '30'),
       ('app.log_level', 'info');