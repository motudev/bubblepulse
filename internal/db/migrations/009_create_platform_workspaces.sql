-- +goose Up
-- Global Directory table: maps an external workspace/tenant identifier
-- (Slack team_id, future Teams/Google/SAML tenant ID) to an organization.
-- Queried before any tenant context exists, so it must never be under RLS.
CREATE TABLE platform_workspaces (
    id          BIGSERIAL    PRIMARY KEY,
    org_id      UUID         NOT NULL,
    provider    TEXT         NOT NULL,
    external_id TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_platform_workspaces_org
        FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT uq_platform_workspaces_provider_external UNIQUE (provider, external_id)
);
CREATE INDEX idx_platform_workspaces_org ON platform_workspaces (org_id);

-- +goose Down
DROP TABLE platform_workspaces;
