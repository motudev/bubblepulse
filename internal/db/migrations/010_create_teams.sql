-- +goose Up
CREATE TABLE teams (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID         NOT NULL,
    name       TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_teams_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX idx_teams_org ON teams (org_id);

ALTER TABLE teams ENABLE ROW LEVEL SECURITY;
-- FORCE is required: the application role owns the tables (goose runs as it),
-- and table owners are exempt from RLS unless forced.
ALTER TABLE teams FORCE ROW LEVEL SECURITY;

-- Rows are visible/editable only for the current tenant, unless the deployment
-- runs as a dedicated single-tenant silo (app.is_siloed = 'true').
-- NULLIF guards the ''::uuid cast error when the GUC is unset; a missing GUC
-- yields NULL and hides every row — fail closed.
CREATE POLICY tenant_isolation_teams ON teams
    USING (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    );

-- +goose Down
DROP POLICY tenant_isolation_teams ON teams;
ALTER TABLE teams NO FORCE ROW LEVEL SECURITY;
ALTER TABLE teams DISABLE ROW LEVEL SECURITY;
DROP TABLE teams;
