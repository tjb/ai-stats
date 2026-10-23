# AI Stats

**Free, open source, self-hosted AI coding tool ROI analytics for engineering leaders.**

Know exactly what your AI coding tools are returning. Connect your GitHub organization, see
real-time acceptance rates across GitHub Copilot, Claude Code, and any custom tool via the push
ingest API, and generate board-ready ROI reports — automatically.

## Features

- **Real-time Dashboard** — Track acceptance rates, active users, and lines accepted
- **ROI Calculation** — Translate accepted code into hours saved and dollar value, with a
  configurable hourly rate
- **Monthly Reports** — Auto-generated, shareable date-range reports for stakeholders
- **Multi-Tool Attribution** — Compare adoption across Copilot, Claude Code, and any custom tool
  via the push ingest API
- **Editor & Language Breakdown** — See which tools and languages drive adoption
- **Planning Ratio** — Classify AI tool usage into planning vs. coding actions

AI Stats is MIT-licensed and designed to run on your own infrastructure with your own database —
no seat limits, no subscription.

## Tech Stack

- **Backend**: Go (chi router, `pgx` for PostgreSQL, hand-written SQL — no ORM/codegen)
- **Frontend**: React 19 + Vite, TypeScript, Tailwind CSS v4
- **Database**: PostgreSQL
- **Auth**: GitHub OAuth (`read:user`, `user:email`, `read:org` scopes), JWT sessions
- **Charts**: Recharts

## Project Structure

```
backend/
├── cmd/server/           # main.go — HTTP server entrypoint
└── internal/
    ├── auth/              # GitHub OAuth + JWT session handling
    ├── crypto/             # AES encryption for stored GitHub tokens
    ├── db/                 # pgx pool, hand-written queries, SQL migrations
    ├── github/              # GitHub Copilot Usage API client + sync
    ├── handlers/            # HTTP handlers (dashboard, insights, reports, settings, ingest...)
    ├── ingest/               # Push-ingest API key generation/hashing
    ├── middleware/           # Auth middleware, security headers
    └── roi/                   # ROI calculation (hours/dollars saved)

frontend/
└── src/
    ├── api/              # API client + response types
    ├── components/       # Layout, charts, dashboard components
    ├── hooks/            # React hooks
    └── pages/            # Route-level pages
```

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20+ and pnpm 9+
- PostgreSQL
- A GitHub OAuth App ([create one here](https://github.com/settings/developers))

### Setup

1. **Clone and install:**

   ```bash
   git clone https://github.com/tjb/ai-stats.git
   cd ai-stats
   cd frontend && pnpm install && cd ..
   ```

2. **Configure the backend:**

   ```bash
   cd backend
   cp .env.example .env
   # Fill in DATABASE_URL, JWT_SECRET, ENCRYPTION_KEY, GITHUB_CLIENT_ID/SECRET
   ```

   Generate `JWT_SECRET` and `ENCRYPTION_KEY` with:

   ```bash
   openssl rand -hex 32
   ```

3. **Configure the frontend (optional):**

   ```bash
   cd frontend
   cp .env.example .env.local
   ```

4. **Run both services:**

   ```bash
   make dev-backend    # Go server on :8080, applies migrations automatically on start
   make dev-frontend   # Vite dev server on :5173, proxies /api to :8080
   ```

### GitHub OAuth App Setup

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Create a new OAuth App
3. Set the callback URL to `http://localhost:8080/api/auth/callback`
4. Copy the Client ID and Client Secret into `backend/.env`

**Required scopes:** `read:user`, `user:email`, `read:org`

### Push Ingest API (optional)

To attribute usage from tools other than GitHub Copilot (e.g. Claude Code), generate an API key
from **Settings → Push Ingest API** in the app, then POST events to `/api/ingest/events`. See
`backend/internal/handlers/ingest.go` for the payload format.

## Building for Production

```bash
make build-backend    # -> backend/bin/server
make build-frontend   # -> frontend/dist
```

Serve `frontend/dist` from any static host or reverse proxy, and point it at the Go server for
`/api/*`.

### Database Migrations

Migrations are plain SQL files in `backend/internal/db/migrations/` and are applied
automatically, in order, the first time the server connects to a fresh (or behind) database. No
separate migration step is required.

## Environment Variables

See `backend/.env.example` and `frontend/.env.example` for all required variables.

| Variable        | Default | Description                                                                                                                                                                         |
| --------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `SYNC_INTERVAL` | `1h`    | How often the backend re-syncs every connected org from GitHub in the background (the same work as **Sync now**). Go duration, minimum `1m`; `0` or `off` disables background sync. |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for local dev workflow, PR expectations, and how
migrations work.

## Security

See [SECURITY.md](SECURITY.md) for how to report a vulnerability.

## License

[MIT](LICENSE)
