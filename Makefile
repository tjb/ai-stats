.PHONY: dev-backend dev-frontend dev build-backend build-frontend lint-backend lint-frontend
.PHONY: fmt-check test-backend lint check

# Run Go backend (requires .env in backend/)
dev-backend:
	cd backend && go run ./cmd/server

# Run Vite dev server (proxies /api to :8080)
dev-frontend:
	cd frontend && pnpm dev

# Run both in parallel (requires 'make -j2' or tmux)
dev:
	@echo "Run: make -j2 dev-backend dev-frontend"

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && pnpm build

lint-backend:
	cd backend && go vet ./...

lint-frontend:
	cd frontend && pnpm build --no-clear

# Fail and list any Go files that need gofmt (same check as CI)
fmt-check:
	@cd backend && unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted (run 'gofmt -w' in backend/):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

# Run Go tests. DB integration tests run only when TEST_DATABASE_URL is set,
# from the environment or as 'make test-backend TEST_DATABASE_URL=postgres://...'
test-backend:
	@if [ -z "$$TEST_DATABASE_URL" ]; then echo "TEST_DATABASE_URL not set; DB integration tests will be skipped"; fi
	cd backend && go test ./...

# go vet + eslint
lint: lint-backend
	cd frontend && pnpm lint

# Everything CI runs: gofmt, vet, tests, eslint, frontend build
check: fmt-check lint test-backend build-frontend
