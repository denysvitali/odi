.PHONY: build build-go build-frontend \
        test test-go test-frontend \
        lint docker-up docker-down

# ── Build ──────────────────────────────────────────────────────────────────────

build: build-go build-frontend

build-go:
	go build ./...

build-frontend:
	cd frontend && pnpm run build

# ── Test ───────────────────────────────────────────────────────────────────────

test: test-go test-frontend

test-go:
	go test ./...

test-frontend:
	cd frontend && pnpm test

# ── Lint ───────────────────────────────────────────────────────────────────────

lint:
	golangci-lint run
	cd frontend && pnpm lint

# ── Docker ─────────────────────────────────────────────────────────────────────

docker-up:
	docker compose up -d

docker-down:
	docker compose down
