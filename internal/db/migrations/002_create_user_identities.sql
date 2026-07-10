-- +goose Up
CREATE TABLE user_identities (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT         NOT NULL,
    provider_id TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_identities_provider UNIQUE (provider, provider_id)
);

-- Speeds up lookups when a user returns via OIDC callback
CREATE INDEX idx_user_identities_lookup ON user_identities (provider, provider_id);

-- +goose Down
DROP TABLE user_identities;
