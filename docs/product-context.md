# Product context

What AI Stats is becoming, the rules every change has to respect, and the current state of the
code. Read this before changing the data model, adding a data source, or adding a metric.

## Direction

AI Stats is an open-source (MIT), self-hosted platform that shows engineering orgs what their AI
coding tools cost and what that spend delivers. It pulls token spend from Claude Code, Cursor,
GitHub Copilot and LLM gateways, joins it to delivery and quality outcomes (merged PRs, deploys,
incidents, churn), and runs entirely inside the customer's own network. Tagline: "Tokens in,
outcomes out."

The pitch rests on being the neutral, auditable, self-hosted option in a market of SaaS products
(Jellyfish, Faros AI, Swarmia, DX). Changes that weaken that position need a strong reason.

## Rules every change respects

- **Metadata only.** Never store prompt text, completions, code, file contents or tool arguments.
  Telemetry and vendor payloads are reduced to counts, tokens, cost, model, timestamps and IDs
  before they reach the database. Error messages and other free-text fields are dropped.
- **Team-level by default.** Manager-facing views aggregate to teams with a minimum group size of
  5, enforced in the query layer, not just the UI. Developers can see their own numbers.
  Per-person views for managers are opt-in by org policy.
- **No rankings.** No leaderboards, "top users" or per-person scores, anywhere. (Meta and Amazon
  both pulled token leaderboards in 2026 after people gamed them.)
- **Self-hosted, one container plus Postgres.** No new required infrastructure (no Kafka,
  ClickHouse, Grafana, Airbyte, dbt). The data never leaves the customer's network.
- **Every metric is plain SQL in the repo,** so anyone can check how a number was made.
- **Estimated vs. invoiced cost are distinct.** Vendor "estimated cost" fields and list-price
  calculations are labeled estimates; never present them as billed amounts.
- **No causal claims.** Show correlation, cohorts and ranges.

## How data gets in

1. **Vendor admin APIs, pulled by the server** (no install on laptops): Anthropic Claude Code
   Analytics API, Cursor Admin API, GitHub Copilot usage metrics API. Specs:
   [docs/integrations](integrations/README.md).
2. **Claude Code's OpenTelemetry export,** configured centrally by IT through managed settings and
   pointed at the org's own AI Stats instance. Spec:
   [claude-code-opentelemetry.md](integrations/claude-code-opentelemetry.md).
3. **LLM gateway logs** (LiteLLM and similar).
4. **Push-ingest API** (`POST /api/ingest/events`) for custom tools and CI scripts.
5. **Outcome sources:** GitHub (PRs, reviews, reverts, deploys), CI/CD, incident tools, issue
   trackers.

**Deprecated:** `frontend/public/install.sh` (per-laptop Claude Code hooks installed with
`curl | bash`) and the Codex shell wrapper in `frontend/src/lib/ingestSnippet.ts`. They send no
tokens or cost, report users as `$USER@local` (nothing joins to GitHub or teams), put one org-wide
key on every laptop, and can drop events. Don't extend them; onboarding moves to the managed OTel
setup.

## Code map

- `backend/cmd/server` — wiring: config, migrations, router, scheduler, SPA serving.
- `backend/internal/db` — pgx pool, hand-written queries, migrations (`migrations/NNN_*.sql`,
  applied in order on startup; the next number is 009).
- `backend/internal/github` — GitHub API clients; `sync.go` is the one sync entry point used by
  both the HTTP handlers and the scheduler.
- `backend/internal/scheduler` — background sync every `SYNC_INTERVAL` (default 1h).
- `backend/internal/handlers` — HTTP handlers; `*_integration_test.go` run against real Postgres.
- `backend/internal/dbtest` — test harness: a fresh database per test from `TEST_DATABASE_URL`.
- `backend/internal/web` — serves the built SPA from `STATIC_DIR` with `index.html` fallback.
- `frontend/` — React 19 + Vite + TypeScript + Tailwind v4.
- `Dockerfile`, `docker-compose.yml` — the one-command self-hosted stack.

Run `make check` before pushing (what CI runs). Backend integration tests need
`TEST_DATABASE_URL` pointing at a Postgres role that can create databases.

## Known constraints in the current data model

Verified during the October 2026 Phase 0 work; account for them when touching these areas.

- **Copilot sync is on a dead endpoint.** `internal/github/copilot.go` calls GitHub's beta
  `/orgs/{org}/copilot/usage`, which has had no new data since February 2025. The replacement is
  report-based; see the Copilot spec.
- **Copilot data and tool data live in different tables.** Copilot sync writes
  `copilot_usage_snapshots`; push ingest and Claude Code attribution write
  `tool_usage_snapshots`. Insights utilization and the tool breakdown read only the latter, so
  Copilot-only orgs (including the demo org) show an empty Insights page. Migration 002 copied
  pre-existing Copilot rows into `tool_usage_snapshots`, and `GetCombinedSnapshots` unions both
  tables, so those days count twice.
- **`UpsertToolSnapshot` adds on conflict.** That suits push ingest but double-counts any connector
  that re-fetches a day. New connectors need replace semantics (or a separate table).
- **Push ingest overwrites `raw_json`** on `claude_code` rows, which wipes the list of
  commit SHAs that Claude Code attribution uses to avoid recounting.
- **Windows are inconsistent.** The dashboard treats the last 30 rows as "the last 30 days"
  (sparse data can span ~42 calendar days); utilization uses calendar windows.
- **Identity hashing.** Push ingest hashes emails with plain unsalted SHA-256 and doesn't
  lowercase first. Use an HMAC with a server secret, on lowercased emails, for new identity code.
- **Claude Code attribution** looks back 24 hours and reads at most 100 commits per repo.
- **Ingest keys** are generated with the prefix `lum_live_`, while docs and the UI say
  `aistats_live_`.

## Roadmap

- **Phase 0 — groundwork (done Oct 2026, except the Copilot migration):** CI, Docker, background
  sync, database integration tests, frontend build fixed.
- **Phase 1 — spend:** token and cost data model (people, teams, daily token usage by tool, model
  and person; estimated vs. invoiced); a connector interface; an OTLP receiver for Claude Code;
  connectors for the Anthropic, Cursor and Copilot APIs and LiteLLM; model price table; token
  fields on the push API; spend views; onboarding switched to the managed OTel setup.
- **Phase 2 — outcomes:** PRs, reviews, deploys and reverts; GitHub teams mapping; AI attribution
  per PR with a confidence level; the six metrics (cost per merged PR, efficiency curve,
  AI-assisted change failure rate, 14-day churn, lead time delta, budget burn and forecast).
- **Phase 3 — trust and launch:** roles (admin, manager, developer), minimum group size in
  queries, pseudonymized IDs and audit log, budget caps and alerts, public dogfood dashboard, open
  data schema on OTel GenAI conventions, works-council privacy document.

Work items are tracked in beads (`bd`), per AGENTS.md.
