# ODI — Open Document Indexer

A privacy-first, self-hosted document digitization system. Scan paper documents with a network scanner, run OCR on-device, and full-text search your archive — without sending data to any cloud service.

## Architecture

```
Scanner (AirScan) ──► Backend ──► OCR Service (Android/ML Kit)
                          │
                          ▼
                     OpenSearch ◄──── Frontend (Vue 3 SPA)
                          │
                     Storage (B2 / Local FS)
```

## Components

| Directory | Description |
|---|---|
| [`backend/`](backend/) | Go REST API — ingestion, OCR orchestration, OpenSearch indexing |
| [`frontend/`](frontend/) | Vue 3 + TypeScript SPA — document search and viewer |
| [`zefix-tools/`](zefix-tools/) | Swiss company database import and lookup (Zefix / SPARQL) |
| [`logo-recognition/`](logo-recognition/) | Experimental TensorFlow/Keras logo classifier |

## Prerequisites

| Tool | Purpose |
|---|---|
| Go 1.23+ | Backend and zefix-tools |
| pnpm | Frontend build |
| Docker + Compose | OpenSearch, PostgreSQL |
| OpenCV | Image processing in backend (`gocv`) |
| OCR service | Remote Android ML Kit server (see backend README) |

Install OpenCV on Linux:
```bash
git clone https://github.com/hybridgroup/gocv && cd gocv && make install
```

## Quick Start

```bash
# 1. Start infrastructure
make docker-up

# 2. Build everything
make build

# 3. Configure the backend
cp backend/.env.example backend/.env
# Edit backend/.env with your OpenSearch, storage, and OCR settings

# 4. Run the backend server
cd backend && go run . serve

# 5. Index an existing folder of scanned images
cd backend && go run . index /path/to/scans

# 6. Index a folder of PDF files
cd backend && go run . pdf /path/to/pdfs

# 7. Open the frontend (after building)
cd frontend && pnpm run dev
```

## Repository Layout

```
odi/
├── backend/              # Go service (main entrypoint)
│   ├── internal/cli/     # Cobra CLI commands
│   ├── pkg/indexer/      # OCR + OpenSearch indexing pipeline
│   ├── pkg/ocrclient/    # HTTP client for the OCR service
│   ├── pkg/storage/      # Pluggable storage (B2, filesystem, rclone)
│   └── pkg/server/       # Gin REST API
├── frontend/             # Vue 3 SPA
├── zefix-tools/          # Swiss company DB tooling
├── logo-recognition/     # Experimental ML classifier
├── docker-compose.yml    # OpenSearch + PostgreSQL
├── Makefile              # Unified build/test/lint targets
└── go.work               # Go workspace (backend + zefix-tools)
```

## Make Targets

```
make build            Build all components
make test             Run all tests
make lint             Run golangci-lint and pnpm lint
make docker-up        Start OpenSearch + PostgreSQL
make docker-down      Stop all containers
make proto            Regenerate protobuf files (backend)
```

## Privacy

All processing runs locally:
- OCR is performed on an Android device on the local network
- Documents are stored in your own B2 bucket (encrypted) or local filesystem
- OpenSearch runs in Docker on your own hardware
- No telemetry, no cloud dependencies
