-- +goose Up
-- Global Directory table: the session row is how the API middleware learns the
-- tenant before opening an RLS-scoped transaction, so no RLS here.
ALTER TABLE sessions ADD COLUMN org_id UUID NULL;
ALTER TABLE sessions
    ADD CONSTRAINT fk_sessions_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE sessions DROP CONSTRAINT fk_sessions_org;
ALTER TABLE sessions DROP COLUMN org_id;
