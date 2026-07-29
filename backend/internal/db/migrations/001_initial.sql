-- Initial schema for AI Stats
-- Translated from prisma/schema.prisma to plain SQL

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id                TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    email             TEXT NOT NULL UNIQUE,
    name              TEXT,
    image             TEXT,
    stripe_customer_id TEXT,
    hourly_rate       INT  NOT NULL DEFAULT 150,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL,
    provider_account_id TEXT NOT NULL,
    access_token        TEXT,
    refresh_token       TEXT,
    expires_at          BIGINT,
    token_type          TEXT,
    scope               TEXT,
    UNIQUE (provider, provider_account_id)
);

CREATE INDEX accounts_user_id_idx ON accounts(user_id);

CREATE TABLE organizations (
    id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    github_org_id         TEXT NOT NULL UNIQUE,
    github_org_login      TEXT NOT NULL,
    name                  TEXT NOT NULL,
    avatar_url            TEXT,
    github_access_token   TEXT NOT NULL,
    plan                  TEXT NOT NULL DEFAULT 'free',
    stripe_subscription_id TEXT,
    owner_id              TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX organizations_owner_id_idx ON organizations(owner_id);

CREATE TABLE copilot_usage_snapshots (
    id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    date                 DATE NOT NULL,
    total_suggestions    INT  NOT NULL DEFAULT 0,
    total_acceptances    INT  NOT NULL DEFAULT 0,
    total_lines_accepted INT  NOT NULL DEFAULT 0,
    total_active_users   INT  NOT NULL DEFAULT 0,
    total_licensed_users INT  NOT NULL DEFAULT 0,
    breakdown_json       JSONB NOT NULL DEFAULT '[]',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, date)
);

CREATE INDEX copilot_usage_snapshots_org_date_idx ON copilot_usage_snapshots(organization_id, date);

CREATE TABLE reports (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    month           TEXT NOT NULL,  -- YYYY-MM format
    content_json    JSONB NOT NULL DEFAULT '{}',
    share_token     TEXT UNIQUE DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, month)
);

CREATE INDEX reports_org_month_idx ON reports(organization_id, month);
