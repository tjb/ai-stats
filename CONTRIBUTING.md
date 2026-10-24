# Contributing to AI Stats

Thanks for your interest in contributing! AI Stats is a self-hosted, open source tool for
measuring AI coding tool ROI, and contributions of all sizes are welcome — bug fixes, new
metrics, documentation, and new tool integrations.

## Project structure

```
backend/    Go API server (chi router, PostgreSQL via pgx)
frontend/   React + Vite SPA (Tailwind CSS, Recharts)
```

## Getting set up

See the [README](README.md#getting-started) for full local setup instructions.

```bash
make dev-backend    # Go server on :8080
make dev-frontend   # Vite dev server on :3000 (proxies /api to :8080)
```

## Before opening a PR

```bash
make check   # gofmt, go vet, eslint, backend tests, frontend build — what CI runs
```

Backend integration tests need a Postgres role that can create databases; point
`TEST_DATABASE_URL` at it (they skip when it is unset):

```bash
make test-backend TEST_DATABASE_URL=postgres://aistats:aistats@localhost:5432/aistats_test?sslmode=disable
```

Keep PRs focused — one logical change per PR makes review much faster. If you're planning a
larger change (a new tool integration, a schema change), open an issue first to discuss the
approach.

## Database migrations

Migrations live in `backend/internal/db/migrations/` as plain numbered `.sql` files and are
applied automatically and sequentially on server start (see `backend/internal/db/db.go`).
Never edit an existing migration that may already be applied somewhere — add a new one instead.

## Commit style

Write commit messages that explain *why* a change was made, not just what changed. Keep the
subject line under ~70 characters.

## Reporting bugs

Open a GitHub issue with steps to reproduce, what you expected, and what actually happened.
For security vulnerabilities, please follow [SECURITY.md](SECURITY.md) instead of opening a
public issue.

## Code of conduct

Be respectful and constructive. We want this to be a welcoming project for contributors of all
experience levels.
