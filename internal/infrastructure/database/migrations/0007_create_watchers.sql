CREATE TABLE watchers
(
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    watcher_user_id BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A watcher can only follow a patient once.
    CONSTRAINT uq_watchers_pair UNIQUE (user_id, watcher_user_id),
    -- A user cannot watch themselves.
    CONSTRAINT chk_watchers_no_self CHECK (user_id <> watcher_user_id)
);

CREATE INDEX idx_watchers_user_id ON watchers (user_id);
CREATE INDEX idx_watchers_watcher_id ON watchers (watcher_user_id);


CREATE TABLE watcher_notifications
(
    id          BIGSERIAL PRIMARY KEY,
    watcher_id  BIGINT      NOT NULL REFERENCES watchers (id) ON DELETE CASCADE,
    reminder_id BIGINT      NOT NULL, -- logical FK (reminder is partitioned)
    event       VARCHAR(32) NOT NULL, -- confirmed | skipped | postponed
    sent_at     TIMESTAMPTZ,          -- NULL = not yet delivered
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_watcher_notifs_watcher_id ON watcher_notifications (watcher_id);
CREATE INDEX idx_watcher_notifs_reminder_id ON watcher_notifications (reminder_id);

-- Worker queue: "pending notifications to send"
CREATE INDEX idx_watcher_notifs_unsent
    ON watcher_notifications (created_at) WHERE sent_at IS NULL;