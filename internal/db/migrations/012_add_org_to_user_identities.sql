-- +goose Up
-- Global Directory table: identity → (user, org) resolution happens before any
-- tenant context exists (auth middleware, webhooks), so no RLS here.
ALTER TABLE user_identities ADD COLUMN org_id UUID NULL;
ALTER TABLE user_identities
    ADD CONSTRAINT fk_user_identities_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE user_identities DROP CONSTRAINT fk_user_identities_org;
ALTER TABLE user_identities DROP COLUMN org_id;
