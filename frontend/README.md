# odi-frontend

## Description

`odi-frontend` is the web UI for [ODI](https://github.com/denysvitali/odi), a privacy-aware tool that scans, OCR-processes, and indexes paper documents. It lets you search and browse digitized documents through the ODI backend's REST API.

![ODI Frontend](https://github.com/denysvitali/odi/raw/main/docs/odi-frontend.jpg)

## Prerequisites

- The ODI backend (`go run . serve` from the repo root) must be running and reachable.
- [pnpm](https://pnpm.io/) for package management.
- Node 20+.

## Running

### Using Docker

The repository ships a `Dockerfile` for the frontend. The unified `docker-compose.yml` at the repo root starts only the data services (OpenSearch, Postgres) — to run the frontend in a container, build the image manually:

```bash
docker build -t odi-frontend .
docker run --rm -p 8080:80 odi-frontend
```

### Development setup

From the repo root (or this directory):

```bash
# Install dependencies
pnpm install

# Start the Vite dev server (http://localhost:5173)
pnpm run dev
```

### Useful scripts

```bash
pnpm run build    # production build
pnpm run lint     # ESLint
pnpm test         # Vitest
```

### Runtime configuration

The SPA reads runtime settings from `public/settings.json` (or `settings.json.tpl` in Docker — substituted at container start). At minimum it needs:

- `apiUrl` — the ODI backend REST endpoint
- `opensearchUrl` — OpenSearch Dashboards URL (used only for deep links)
