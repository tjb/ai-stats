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

### Quick start (Docker)

The fastest way to run AI Stats is the bundled Docker Compose stack: one app container (the Go
API serving the built React app on the same origin) plus PostgreSQL. All you need is Docker
with Compose v2.

1. **Create a GitHub OAuth App** at [GitHub Developer Settings](https://github.com/settings/developers)
   → *OAuth Apps* → *New OAuth App*:
   - Homepage URL: `http://localhost:8080`
   - Authorization callback URL: `http://localhost:8080/api/auth/callback`

2. **Configure:**

   ```bash
   git clone https://github.com/tjb/ai-stats.git
   cd ai-stats
   cp .env.example .env
   ```

   Then edit `.env`: paste the OAuth app's `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`, and
   set `JWT_SECRET` and `ENCRYPTION_KEY` to two separate values from:

   ```bash
   openssl rand -hex 32
   ```

   Change `POSTGRES_PASSWORD` too if you like, before the first start.

3. **Start it:**

   ```bash
   docker compose up
   ```

   The first run builds the image (a few minutes). Add `-d` to run in the background, and
   `--build` after pulling new code to rebuild.

4. **Open [http://localhost:8080](http://localhost:8080)** and sign in with GitHub.

Database migrations run automatically on startup and data is kept in the `db-data` volume.
`docker compose logs -f app` shows the server log; `docker compose down` stops everything
(add `-v` to also delete the data). To run on a real hostname, set `APP_URL` in `.env` to the
public URL (e.g. `https://ai-stats.example.com` behind your TLS reverse proxy) and update the
OAuth app's callback URL to match.

### Manual setup (for development)

#### Prerequisites

- Go 1.25+
- Node.js 20+ and pnpm 9+
- PostgreSQL
- A GitHub OAuth App ([create one here](https://github.com/settings/developers))

#### Setup

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
   # Fill in JWT_SECRET, ENCRYPTION_KEY, GITHUB_CLIENT_ID/SECRET; check DATABASE_URL
   ```

   Generate `JWT_SECRET` and `ENCRYPTION_KEY` with:

   ```bash
   openssl rand -hex 32
   ```

3. **Configure the frontend (optional):** to click through the UI with mock data and no
   backend, put `VITE_USE_MOCK=true` in `frontend/.env.local`.

4. **Run both services:**

   ```bash
   make dev-backend    # Go server on :8080, applies migrations automatically on start
   make dev-frontend   # Vite dev server on :3000, proxies /api to :8080
   ```

### GitHub OAuth App Setup

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Create a new OAuth App
3. Set the callback URL to `<APP_URL>/api/auth/callback`: `http://localhost:8080/api/auth/callback`
   for Docker, `http://localhost:3000/api/auth/callback` for the manual setup (the Vite dev server
   proxies `/api` to the Go server)
4. Copy the Client ID and Client Secret into `.env` (Docker) or `backend/.env` (manual setup)

**Required scopes:** `read:user`, `user:email`, `read:org`

### Push Ingest API (optional)

To attribute usage from tools other than GitHub Copilot (e.g. Claude Code), generate an API key
from **Settings → Push Ingest API** in the app, then POST events to `/api/ingest/events`. See
`backend/internal/handlers/ingest.go` for the payload format.

## Building for Production

The `Dockerfile` builds a single minimal image (distroless, non-root) containing the server
binary and the built frontend:

```bash
docker build -t ai-stats .
docker run --env-file .env -e DATABASE_URL=postgres://... -p 8080:8080 ai-stats
```

The image has no shell, so it has no `HEALTHCHECK`; point your orchestrator's HTTP probe at
`GET /health`.

Without Docker:

```bash
make build-backend    # -> backend/bin/server
make build-frontend   # -> frontend/dist
```

Set `STATIC_DIR` to the built `frontend/dist` directory and the Go server serves the frontend
itself, on the same origin as the API: client-side routes fall back to `index.html`, while
`/api/*` and `/health` never do. Alternatively, serve `frontend/dist` from any static host or
reverse proxy and point it at the Go server for `/api/*`.

### Database Migrations

Migrations are plain SQL files in `backend/internal/db/migrations/` and are applied
automatically, in order, the first time the server connects to a fresh (or behind) database. No
separate migration step is required.

## Environment Variables

See [`.env.example`](.env.example) (Docker) and [`backend/.env.example`](backend/.env.example)
(manual setup) for every variable the server reads, with comments. Required:
`APP_URL`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `JWT_SECRET`, `ENCRYPTION_KEY` and
`DATABASE_URL` (set for you by `docker-compose.yml`). Optional: `PORT` (default `8080`) and
`STATIC_DIR` (directory of the built frontend to serve; set in the Docker image). The frontend
reads only `VITE_USE_MOCK`.

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
