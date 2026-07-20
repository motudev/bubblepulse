-- +goose Up
ALTER TABLE daily_update_topics ADD COLUMN org_id UUID NULL;
ALTER TABLE daily_update_topics
    ADD CONSTRAINT fk_daily_update_topics_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

CREATE INDEX idx_daily_update_topics_org ON daily_update_topics (org_id);

ALTER TABLE daily_update_topics ENABLE ROW LEVEL SECURITY;
ALTER TABLE daily_update_topics FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_daily_update_topics ON daily_update_topics
    USING (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    );

-- +goose Down
DROP POLICY tenant_isolation_daily_update_topics ON daily_update_topics;
ALTER TABLE daily_update_topics NO FORCE ROW LEVEL SECURITY;
ALTER TABLE daily_update_topics DISABLE ROW LEVEL SECURITY;
DROP INDEX idx_daily_update_topics_org;
ALTER TABLE daily_update_topics DROP CONSTRAINT fk_daily_update_topics_org;
ALTER TABLE daily_update_topics DROP COLUMN org_id;
