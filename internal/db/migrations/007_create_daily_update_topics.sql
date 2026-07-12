-- +goose Up
CREATE TABLE daily_update_topics (
    id               BIGSERIAL    PRIMARY KEY,
    daily_update_id  BIGINT       NOT NULL,
    extracted_topic  TEXT         NOT NULL,
    topic_embedding  vector(384)  NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_daily_update_topics_update
        FOREIGN KEY (daily_update_id) REFERENCES daily_updates(id) ON DELETE CASCADE
);

CREATE INDEX idx_daily_update_topics_update_id
    ON daily_update_topics (daily_update_id);

CREATE INDEX idx_daily_update_topics_embedding
    ON daily_update_topics USING ivfflat (topic_embedding vector_cosine_ops)
    WITH (lists = 10)
    WHERE topic_embedding IS NOT NULL;

-- +goose Down
DROP TABLE daily_update_topics;
