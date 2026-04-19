## CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Layout

Monorepo with a single Go module (`github.com/denysvitali/odi`) and a Vue 3 frontend, unified through `Makefile` targets.

- `main.go` — Go CLI entry point (Cobra), binary name `odi`
- `internal/cli/` — CLI command handlers (serve, index, pdf, ingest, zefix-import, zefix-find, etc.)
- `internal/server/` — Gin REST API
- `pkg/` — Shared Go packages (indexer, ingestor, storage, ocrclient, models, crypt, zefix, etc.)
- `zefix-tools/` — Zefix Swiss company register package (`pkg/zefix/`) and legacy standalone CLIs
- `frontend/` — Vue 3 + TypeScript SPA built with Vite. See `frontend/CLAUDE.md`.
- `helm/odi/` — Helm chart for deployment.

All Go build/test/lint commands run from the repo root.

## Common Commands

```bash
make build          # builds Go binary and frontend
make test           # runs go test ./... and pnpm test
make lint           # golangci-lint + pnpm lint
make docker-up      # starts OpenSearch, OpenSearch Dashboards, and PostgreSQL
make docker-down
```

Component-specific:

```bash
# Go CLI (from repo root)
go run . serve                                 # REST API
go run . index /path/to/scans                  # Index a directory of images
go run . pdf /path/to/pdfs                     # Index PDFs
go run . zefix-import -i zefix.json -d $ZEFIX_DSN  # Import Zefix data
go run . zefix-find "Company Name" -d $ZEFIX_DSN   # Find a company
go test ./pkg/crypt -run TestEncryptDecrypt    # Single test
E2E_TEST=1 go test ./...                       # E2E tests (gated by env var)

# Frontend (from frontend/)
pnpm run dev
pnpm test -- path/to/file.spec.ts              # Single test
```

`make docker-up` reads `.env` in the repo root and requires `OPENSEARCH_ADMIN_PASSWORD` (see `.env.example`).

## Prerequisites

- Go 1.26+
- pnpm for the frontend
- Docker + Compose
- A reachable [ocr-server](https://github.com/denysvitali/ocr-server) (Android/ML Kit) for OCR
- Optional: an AirScan/eSCL scanner for live ingestion

## Architecture (big picture)

```
Scanner (AirScan) / File Upload ──► pkg/ingestor / internal/server ──► pkg/indexer ──► OpenSearch
                                                                              │
                                                                              ├──► pkg/ocrclient ──► OCR server (remote)
                                                                              └──► pkg/storage    ──► B2 (encrypted) or local FS
                                                                                                       │
Vue 3 SPA (frontend/) ──► internal/server (Gin REST) ◄────────────────────────── OpenSearch
                                        │
                                        └──► pkg/zefix (PostgreSQL from zefix-tools import)
```

Key cross-component contracts:
- `STORAGE_TYPE` (`b2` or `filesystem`), `OPENSEARCH_*`, `OCR_API_ADDR`, `ZEFIX_DSN`, `SCANNER_NAME` — consumed by the server/indexer.
- Credentials may use a `keychain:<name>` prefix (e.g. `B2_KEY=keychain:b2-key`) for OS keychain lookup.
- B2 storage is AES-256-GCM encrypted with PBKDF2 (`pkg/crypt`); filesystem storage is plaintext — use a FUSE-encrypted mount if you need at-rest encryption there.
- The frontend loads runtime config from `public/settings.json` (templated by `settings.json.tpl` in Docker).
- CORS origins for the backend are set via `CORS_ALLOWED_ORIGINS` (default `http://localhost:5173`).

## CI

`.github/workflows/ci.yml` is the single CI entry point — consult it before adding lint/test steps elsewhere.
