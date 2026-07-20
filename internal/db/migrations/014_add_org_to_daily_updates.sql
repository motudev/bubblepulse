-- +goose Up
ALTER TABLE daily_updates ADD COLUMN org_id UUID NULL;
ALTER TABLE daily_updates
    ADD CONSTRAINT fk_daily_updates_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

CREATE INDEX idx_daily_updates_org ON daily_updates (org_id, created_at DESC)
    WHERE deleted_at IS NULL;

ALTER TABLE daily_updates ENABLE ROW LEVEL SECURITY;
ALTER TABLE daily_updates FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_daily_updates ON daily_updates
    USING (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    );

-- +goose Down
DROP POLICY tenant_isolation_daily_updates ON daily_updates;
ALTER TABLE daily_updates NO FORCE ROW LEVEL SECURITY;
ALTER TABLE daily_updates DISABLE ROW LEVEL SECURITY;
DROP INDEX idx_daily_updates_org;
ALTER TABLE daily_updates DROP CONSTRAINT fk_daily_updates_org;
ALTER TABLE daily_updates DROP COLUMN org_id;
