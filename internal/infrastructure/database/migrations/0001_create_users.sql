CREATE TABLE users
(
    id         BIGSERIAL PRIMARY KEY,
    timezone   VARCHAR(64) NOT NULL DEFAULT 'UTC',
    language   VARCHAR(8)  NOT NULL DEFAULT 'en',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_contacts
(
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    messenger_type    VARCHAR(32)  NOT NULL,
    messenger_user_id VARCHAR(128) NOT NULL,
    username          VARCHAR(128),
    is_primary        BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- A user can only have one contact per messenger account.
CREATE UNIQUE INDEX uidx_user_contacts_messenger
    ON user_contacts (messenger_type, messenger_user_id);

-- Fast lookup: "find user by telegram_id"
CREATE INDEX idx_user_contacts_user_id
    ON user_contacts (user_id);