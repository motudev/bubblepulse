-- +goose Up
CREATE TABLE daily_updates (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL,
    update_text TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ NULL,
    CONSTRAINT fk_daily_updates_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_daily_updates_user_active ON daily_updates (user_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE daily_updates;
