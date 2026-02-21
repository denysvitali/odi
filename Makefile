.PHONY: build build-backend build-zefix-tools build-frontend \
        test test-go test-frontend \
        lint docker-up docker-down

# ── Build ──────────────────────────────────────────────────────────────────────

build: build-backend build-zefix-tools build-frontend

build-backend:
	cd backend && go build ./...

build-zefix-tools:
	cd zefix-tools && go build ./...

build-frontend:
	cd frontend && pnpm run build

# ── Test ───────────────────────────────────────────────────────────────────────

test: test-go test-frontend

test-go:
	cd backend && go test ./...
	cd zefix-tools && go test ./...

test-frontend:
	cd frontend && pnpm test

# ── Lint ───────────────────────────────────────────────────────────────────────

lint:
	cd backend && golangci-lint run
	cd zefix-tools && golangci-lint run
	cd frontend && pnpm lint

# ── Docker ─────────────────────────────────────────────────────────────────────

docker-up:
	docker compose up -d

docker-down:
	docker compose down

