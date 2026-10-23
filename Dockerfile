# syntax=docker/dockerfile:1

# AI Stats: one image with the Go API server and the built React SPA, served
# from the same origin. Run it with `docker compose up` (see docker-compose.yml)
# or build it standalone with `docker build -t ai-stats .`.
#
# There is no HEALTHCHECK: the distroless runtime has no shell or curl. Point
# your orchestrator's HTTP probe at GET /health (200 "ok") instead.

# ---- 1. Frontend: React + Vite -> frontend/dist ------------------------------
# Built on the build host's platform: the output is plain static files.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
ARG PNPM_VERSION=10.28.0
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate
WORKDIR /src/frontend
# Dependencies first, so this layer stays cached until the lockfile changes.
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

# ---- 2. Backend: static Go binary --------------------------------------------
# Cross-compiles for the target platform, so multi-arch builds stay fast.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
WORKDIR /src/backend
# Modules first, so this layer stays cached until go.mod/go.sum change.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server

# ---- 3. Runtime: distroless, non-root -----------------------------------------
# static-debian12 ships CA certificates (for the GitHub and Anthropic APIs) and
# tzdata, and nothing else: no shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=frontend /src/frontend/dist /app/web
ENV STATIC_DIR=/app/web \
    PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
