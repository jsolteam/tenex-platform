-- schedule_type:
--   daily    — every day at given times[]
--   weekly   — days_of_week bitmask (1=Mon … 64=Sun) at given times[]
--   interval — every interval_days days at given times[]
--
-- days_of_week bitmask (SMALLINT):
--   1   monday
--   2   tuesday
--   4   wednesday
--   8   thursday
--   16  friday
--   32  saturday
--   64  sunday

CREATE TABLE schedules
(
    id            BIGSERIAL PRIMARY KEY,
    medicine_id   BIGINT      NOT NULL REFERENCES medicines (id) ON DELETE CASCADE,
    schedule_type VARCHAR(32) NOT NULL,  -- daily | weekly | interval
    interval_days SMALLINT,              -- used when type = interval
    days_of_week  SMALLINT,              -- bitmask, used when type = weekly
    times         TIME[]      NOT NULL,  -- array of wall-clock times, e.g. {08:00,20:00}
    start_date    DATE        NOT NULL,
    end_date      DATE,                  -- NULL means no end
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_schedules_medicine_id ON schedules (medicine_id);

CREATE INDEX idx_schedules_date_range ON schedules (start_date, end_date);

CREATE INDEX idx_schedules_no_end ON schedules (start_date) WHERE end_date IS NULL;