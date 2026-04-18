## CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Layout

Monorepo containing three buildable components, unified through `Makefile` targets at the root.

- `backend/` — Go REST API and CLI (Cobra). Entry point: `backend/main.go` → `internal/cli`. See `backend/CLAUDE.md` for package-level details.
- `frontend/` — Vue 3 + TypeScript SPA built with Vite. See `frontend/CLAUDE.md`.
- `zefix-tools/` — Go CLIs (`zefix-import`, `zefix-find`) that ingest and query the Swiss Zefix company database into PostgreSQL; backend depends on this data via `ZEFIX_DSN`.
- `helm/odi/` — Helm chart for deployment.

Go uses a workspace (`go.work`) spanning `./backend` and `./zefix-tools` — build/test commands must be run from those subdirectories, not from the root.

## Common Commands

From the repo root:

```bash
make build          # builds backend, zefix-tools, and frontend
make test           # runs go test ./... in backend & zefix-tools, plus pnpm test
make lint           # golangci-lint on both Go modules, pnpm lint on frontend
make docker-up      # starts OpenSearch, OpenSearch Dashboards, and PostgreSQL
make docker-down
```

Component-specific:

```bash
# Backend (from backend/)
go run . serve                                 # REST API
go run . index /path/to/scans                  # Index a directory of images
go run . pdf /path/to/pdfs                     # Index PDFs
go test ./pkg/crypt -run TestEncryptDecrypt    # Single test
E2E_TEST=1 go test ./...                       # E2E tests (gated by env var)
docker run --rm -v $(pwd):/workspace -w /workspace bufbuild/buf:latest generate  # protobuf

# Frontend (from frontend/)
pnpm run dev
pnpm test -- path/to/file.spec.ts              # Single test
```

`make docker-up` reads `.env` in the repo root and requires `OPENSEARCH_ADMIN_PASSWORD` (see `.env.example`).

## Prerequisites

- Go 1.23+ (workspace pins 1.23.1)
- pnpm for the frontend
- Docker + Compose
- OpenCV — required by `gocv` in the backend for image processing. On Linux: `git clone https://github.com/hybridgroup/gocv && cd gocv && make install`.
- A reachable [ocr-server](https://github.com/denysvitali/ocr-server) (Android/ML Kit) for OCR
- Optional: an AirScan/eSCL scanner for live ingestion

## Architecture (big picture)

```
Scanner (AirScan) ──► backend/pkg/ingestor ──► backend/pkg/indexer ──► OpenSearch
                                                      │
                                                      ├──► backend/pkg/ocrclient ──► OCR server (remote)
                                                      └──► backend/pkg/storage    ──► B2 (encrypted) or local FS
                                                                                       │
Vue 3 SPA (frontend/) ──► backend/pkg/server (Gin REST) ◄────────────────────────── OpenSearch
                                        │
                                        └──► backend/pkg/zefix (PostgreSQL from zefix-tools import)
```

Key cross-component contracts:
- `STORAGE_TYPE` (`b2` or `filesystem`), `OPENSEARCH_*`, `OCR_API_ADDR`, `ZEFIX_DSN`, `SCANNER_NAME` — consumed by the backend; see `backend/README.md` for the full list.
- Credentials may use a `keychain:<name>` prefix (e.g. `B2_KEY=keychain:b2-key`) for OS keychain lookup.
- B2 storage is AES-256-GCM encrypted with PBKDF2 (`backend/pkg/crypt`); filesystem storage is plaintext — use a FUSE-encrypted mount if you need at-rest encryption there.
- The frontend loads runtime config from `public/settings.json` (templated by `settings.json.tpl` in Docker).
- CORS origins for the backend are set via `CORS_ALLOWED_ORIGINS` (default `http://localhost:5173`).

## CI

`.github/workflows/ci.yml` is the single CI entry point — consult it before adding lint/test steps elsewhere.
