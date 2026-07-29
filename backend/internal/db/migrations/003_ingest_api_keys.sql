CREATE TABLE ingest_api_keys (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash        TEXT NOT NULL UNIQUE,  -- SHA-256 hex of the raw key
    label           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);

CREATE INDEX ingest_api_keys_org_idx  ON ingest_api_keys(organization_id);
CREATE INDEX ingest_api_keys_hash_idx ON ingest_api_keys(key_hash);
