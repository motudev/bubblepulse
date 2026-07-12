-- +goose Up
ALTER TABLE daily_updates ADD COLUMN update_embedding vector(384) NULL;

-- +goose Down
ALTER TABLE daily_updates DROP COLUMN update_embedding;
