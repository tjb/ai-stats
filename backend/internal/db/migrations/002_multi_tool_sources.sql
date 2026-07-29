-- Generic multi-tool usage table
CREATE TABLE tool_usage_snapshots (
    id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    tool_source          TEXT NOT NULL,  -- 'copilot', 'claude_code'
    date                 DATE NOT NULL,
    total_suggestions    INT  NOT NULL DEFAULT 0,
    total_acceptances    INT  NOT NULL DEFAULT 0,
    total_lines_accepted INT  NOT NULL DEFAULT 0,
    total_active_users   INT  NOT NULL DEFAULT 0,
    total_licensed_users INT  NOT NULL DEFAULT 0,
    raw_json             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, tool_source, date)
);

CREATE INDEX tool_usage_snapshots_org_source_date_idx
    ON tool_usage_snapshots(organization_id, tool_source, date);

-- Backfill Copilot data from the existing table
INSERT INTO tool_usage_snapshots
    (id, organization_id, tool_source, date,
     total_suggestions, total_acceptances, total_lines_accepted,
     total_active_users, total_licensed_users, raw_json, created_at)
SELECT id, organization_id, 'copilot', date,
    total_suggestions, total_acceptances, total_lines_accepted,
    total_active_users, total_licensed_users, breakdown_json, created_at
FROM copilot_usage_snapshots;
