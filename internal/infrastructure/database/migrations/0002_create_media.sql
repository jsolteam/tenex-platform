CREATE TABLE media
(
    id            BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    media_type    VARCHAR(32) NOT NULL, -- photo | video | document
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_media_owner ON media (owner_user_id);

CREATE TABLE media_variants
(
    id           BIGSERIAL PRIMARY KEY,
    media_id     BIGINT      NOT NULL REFERENCES media (id) ON DELETE CASCADE,
    storage_type VARCHAR(32) NOT NULL, -- messenger | s3 | url
    messenger    VARCHAR(32),          -- telegram | vk | whatsapp  (NULL for s3/url)
    external_id  TEXT,                 -- FileID, S3 key, etc.
    url          TEXT,                 -- public URL if available
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_media_variants_media_id ON media_variants (media_id);

-- Quick lookup: "give me the Telegram variant for media_id=X"
CREATE INDEX idx_media_variants_storage
    ON media_variants (media_id, storage_type, messenger);