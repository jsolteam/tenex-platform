CREATE TABLE medicines
(
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name           VARCHAR(256) NOT NULL,
    photo_media_id BIGINT       REFERENCES media (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_medicines_user_id ON medicines (user_id);