.PHONY: dev-backend dev-frontend dev build-backend build-frontend lint-backend lint-frontend

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
