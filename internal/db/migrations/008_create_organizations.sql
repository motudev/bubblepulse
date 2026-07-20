-- +goose Up
CREATE TABLE organizations (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE organizations;
