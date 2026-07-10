-- +goose Up
CREATE TABLE sessions (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ  NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    CONSTRAINT uq_sessions_token UNIQUE (token)
);

-- Speeds up session validation middleware on every HTTP API request
CREATE INDEX idx_sessions_expiry ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
