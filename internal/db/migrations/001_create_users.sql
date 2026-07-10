-- +goose Up
CREATE TABLE users (
    id         BIGSERIAL    PRIMARY KEY,
    email      TEXT         NOT NULL,
    name       TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_users_email UNIQUE (email)
);

-- +goose Down
DROP TABLE users;
