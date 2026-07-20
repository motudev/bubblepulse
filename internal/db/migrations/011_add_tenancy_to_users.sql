-- +goose Up
ALTER TABLE users
    ADD COLUMN org_id  UUID        NULL,
    ADD COLUMN team_id UUID        NULL,
    ADD COLUMN role    VARCHAR(20) NOT NULL DEFAULT 'UPDATER';

ALTER TABLE users
    ADD CONSTRAINT fk_users_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE users
    ADD CONSTRAINT fk_users_team
        FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE users
    ADD CONSTRAINT ck_users_role
        CHECK (role IN ('ADMIN', 'TEAM_EDITOR', 'UPDATER'));

-- Global email uniqueness is incompatible with pooled multi-tenancy:
-- the same person may exist in two organizations.
ALTER TABLE users DROP CONSTRAINT uq_users_email;
ALTER TABLE users ADD CONSTRAINT uq_users_org_email UNIQUE (org_id, email);

CREATE INDEX idx_users_org ON users (org_id);
CREATE INDEX idx_users_team ON users (team_id);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_users ON users
    USING (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.is_siloed', true) = 'true'
        OR org_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    );

-- +goose Down
DROP POLICY tenant_isolation_users ON users;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
DROP INDEX idx_users_team;
DROP INDEX idx_users_org;
ALTER TABLE users DROP CONSTRAINT uq_users_org_email;
ALTER TABLE users ADD CONSTRAINT uq_users_email UNIQUE (email);
ALTER TABLE users DROP CONSTRAINT ck_users_role;
ALTER TABLE users DROP CONSTRAINT fk_users_team;
ALTER TABLE users DROP CONSTRAINT fk_users_org;
ALTER TABLE users
    DROP COLUMN role,
    DROP COLUMN team_id,
    DROP COLUMN org_id;
