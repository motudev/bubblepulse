-- +goose Up
ALTER TABLE platform_workspaces
    ADD COLUMN bot_token TEXT,
    ADD COLUMN team_name TEXT;

-- +goose Down
ALTER TABLE platform_workspaces
    DROP COLUMN bot_token,
    DROP COLUMN team_name;
