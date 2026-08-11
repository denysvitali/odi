# AGENTS.md

These instructions apply to the entire repository. ODI is a privacy-first document indexer: a
Go CLI/API performs ingestion, OCR orchestration, indexing, and storage, while a Vue SPA provides
the search and document-management UI.

## Start Here

- Read `README.md` for the product model and setup, then inspect the code that owns the feature.
  Do not treat `CLAUDE.md` or `frontend/CLAUDE.md` as more authoritative than source, manifests,
  tests, and workflows.
- This repository contains one Go module rooted at `go.mod` and one pnpm workspace in
  `frontend/`. Run Go commands from the repository root and frontend commands from `frontend/`.
- Before editing, check `git status --short` and preserve unrelated worktree changes.
- Use `rg`/`rg --files` to navigate. Useful entry-point searches are:

  ```bash
  rg 'rootCmd.AddCommand' internal/cli
  rg 'g\.(GET|POST|DELETE)' internal/server/server.go
  rg 'api\.' frontend/src
  ```

## Repository Map

- `main.go` delegates to the Cobra application in `internal/cli/`.
- `internal/cli/root.go` registers commands; `flags.go` owns shared flag/environment wiring;
  command files assemble dependencies for each operation.
- `internal/server/server.go:initRoutes` is the authoritative public HTTP route table. Feature
  handlers and their tests are colocated in `internal/server/`.
- `pkg/ingestor/` owns local and remote ingestion. Keep both implementations of
  `ingestor.Backend` behaviorally compatible.
- `pkg/indexer/` coordinates OCR, metadata extraction, optional LLM/Zefix enrichment, and
  OpenSearch writes.
- `pkg/models/` contains shared domain types. Storage contracts live in `pkg/storage/model/`,
  with implementations in `pkg/storage/fs/`, `pkg/storage/b2/`, and `pkg/storage/rclone/`.
  Depend on the narrow storage capability interface instead of a concrete backend.
- `pkg/ocrclient/` is the OCR transport; `pkg/ocrtext/` extracts text; `pkg/crypt/` contains B2
  blob encryption; `pkg/reindex/` owns reindexing; `pkg/llm/` wraps optional local LLM features.
- `zefix-tools/cmd/` contains legacy standalone commands, but `zefix-tools/pkg/zefix/` is still a
  live library imported by the main application. Do not remove or rewrite the whole subtree as
  dead code.
- `frontend/src/api/client.ts` is the SPA's REST boundary. Views compose feature composables;
  shared transport/domain types live under `frontend/src/types/`; Pinia stores live under
  `frontend/src/stores/`; routing lives in `frontend/src/router/index.ts`.
- `docs/openapi.yaml` documents the API. `.github/workflows/` is the source of truth for CI,
  images, CodeQL, and Pages behavior.

## Architectural Invariants

- The frontend gets application data only through `/api/v1`. `opensearchUrl` is for OpenSearch
  Dashboards deep links, never direct data access.
- `internal/server/server.go:initRoutes` determines whether a handler is reachable. Adding a
  handler file alone does not expose an endpoint.
- Frontend runtime configuration is loaded before Vue mounts into `window._settings`. Local
  values come from `frontend/public/settings.json`; the container renders `settings.json` from
  `settings.json.tpl` in `frontend/docker/entrypoint.sh`. Keep both paths functional.
- Local ingestion reserves a content digest before optional blob storage and indexing. Preserve
  deduplication and partial-failure semantics when changing pipeline order.
- B2 storage can encrypt blobs through `pkg/crypt` when a passphrase is configured; filesystem
  storage is plaintext. Never silently downgrade encryption or imply that every backend provides
  at-rest encryption.
- Optional integrations (OCR server, OpenSearch, scanner, B2, PostgreSQL/Zefix, local LLM) must
  remain mockable or gated in ordinary tests. Do not make the default test suite require live
  infrastructure.

## Common Commands

Prerequisites are Go 1.26.3 or compatible 1.26.x, Node, pnpm, and Docker Compose for local
services. Install frontend dependencies before using root targets that include the SPA:

```bash
cd frontend && pnpm install --frozen-lockfile
```

From the repository root:

```bash
make build             # go build ./... + frontend type-check/build
make test              # go test ./... + Vitest run
make lint              # golangci-lint + ESLint
make docker-up         # OpenSearch, Dashboards, PostgreSQL; reads root .env
make docker-down
go run . --help
go run . serve
```

Focused checks:

```bash
go test ./internal/server
go test ./pkg/indexer -run 'TestName'
cd frontend && pnpm test -- src/path/to/file.spec.ts
cd frontend && pnpm run type-check
cd frontend && pnpm run lint
```

CI is stricter than the Make shortcuts. For broad or risky Go changes, run its equivalents:

```bash
go build ./...
go test -race ./...
golangci-lint run --timeout=5m
```

The CI workflow also runs `govulncheck ./...`. The frontend CI install uses
`pnpm install --frozen-lockfile --ignore-scripts`, followed by lint, build, and tests. Do not
casually normalize tool versions: `go.mod`, `frontend/package.json`, Dockerfiles, and workflows
currently pin or select versions independently; align every consumer in an intentional toolchain
change.

## Testing Expectations

- Add or update tests with behavior changes. Go tests are adjacent `*_test.go` files and commonly
  use `testify`, `httptest`, or `gock`; frontend tests are `*.spec.ts` under nearby `__tests__/`
  directories and run in Vitest's `jsdom` environment.
- Prefer focused package/spec tests while iterating, then run the relevant root checks. Use
  `go test -race ./...` for concurrency, workers, caches, server lifecycle, or shared state.
- Tests requiring real services must keep their existing gates. End-to-end cases check
  `E2E_TEST=true`; some OCR/scanner/OpenSearch cases also require their service-specific
  environment variables. Never put real credentials in tests or fixtures.
- Exercise success, invalid input, upstream failure, and cancellation/cleanup paths for HTTP and
  pipeline changes. Use `httptest` for backend dependencies rather than local network services.
- `pnpm run build` includes Vue TypeScript checking; a Vite bundle alone is not sufficient
  frontend verification.

## Change Checklists

For a public API change, update all affected layers:

1. Register or change the route in `internal/server/server.go:initRoutes`.
2. Update the handler and adjacent Go tests, including authentication and error behavior.
3. Update `docs/openapi.yaml`.
4. Update `frontend/src/api/client.ts`, relevant TypeScript types, and the consuming
   composable/store/view.
5. Add or update frontend specs.

For a CLI or configuration change:

- Register Cobra commands in `internal/cli/root.go`; define shared flags and bindings in
  `internal/cli/flags.go`; preserve the documented flag > environment/config > default behavior.
- Update `.env.example`, README/help text, Docker Compose, and deployment templates when they
  consume the setting. The application supports `ODI_`-prefixed variables plus selected legacy
  unprefixed names; do not silently break either path.
- Keep secrets out of defaults, logs, test snapshots, and commits. `.env` is intentionally
  ignored; edit `.env.example` with placeholders only.

For dependency or container changes:

- Go dependency edits must leave `go.mod` and `go.sum` tidy. Frontend dependency edits must commit
  `frontend/package.json` and `frontend/pnpm-lock.yaml` together.
- Build both affected image stages locally when changing Dockerfiles or entrypoints. The backend
  runtime is distroless and has no shell or curl; health is provided by `/healthz` and `/readyz`.
- Treat `v*` tags and pushes to `main` as publishing actions: workflows build/push GHCR images,
  and `main` also deploys GitHub Pages.

## Code Style

- Format Go with `gofmt`; keep imports idiomatic; wrap errors with operation context and `%w`;
  propagate `context.Context` through I/O boundaries; close response bodies and other resources.
  Follow existing `logrus` structured logging and avoid logging document contents or secrets.
- Keep constructors/options and interfaces consistent with the surrounding package. Add a new
  abstraction only when more than one implementation or a clear test seam needs it.
- Frontend code uses Vue 3 Composition API with `<script setup lang="ts">`, strict TypeScript,
  and `@` aliases from `frontend/vite.config.ts`. Reuse the API client, composables, shared types,
  `components/ui` primitives, `cn()`, Lucide icons, and design tokens instead of issuing ad hoc
  fetches or duplicating UI patterns. Preserve keyboard access, ARIA labeling, focus behavior,
  and reduced-motion support.
- Prettier uses no semicolons, single quotes, two spaces, 100-column width, and no trailing
  commas. Run `pnpm run format` only for intentional `frontend/src/` formatting; avoid unrelated
  bulk churn.
- Treat OCR text, filenames, API payloads, settings, and LLM output as untrusted. Preserve URL
  validation, `encodeURIComponent`, Vue escaping, and DOMPurify/text-only rendering boundaries.

## Security and Data Safety

- Scans, PDFs, OCR text, search/chat terms, extracted metadata, exports, and LLM input/output are
  sensitive untrusted data. Use synthetic fixtures and log identifiers/counts/status only—never
  document content, queries, credentials, tokens, passphrases, authorization headers, or raw
  upstream bodies.
- Keep sensitive API routes inside the authenticated `/api/v1` group. The health, readiness,
  metrics, and tokenized share routes are intentionally public; keep their responses free of
  secrets and internal dependency details. CORS is not authentication.
- The frontend stores the API bearer token in `localStorage`. Do not add raw HTML or untrusted
  scripts, and keep authenticated API-client requests relative to the configured API; never send
  its headers to arbitrary absolute URLs or origins.
- Preserve scan-ID validation, upload size/MIME/worker bounds, outbound URL and SSRF checks,
  storage atomicity, authenticated encryption, share-token expiry/view-limit checks, and LLM
  input/output limits. Do not expand document egress to an external OCR/LLM service without the
  user's explicit authorization and matching privacy documentation.
- Treat Docker startup, imports, reindexing, watcher moves, storage writes/deletes, and real E2E
  tests as stateful operations. Confirm the target and use disposable data; do not point tests at
  a real bucket, database, scanner, or document archive without explicit authorization.
- Before committing, inspect staged and untracked files for `.env`, private keys, credentials,
  document blobs, decrypted exports, local `data/`, logs, and generated artifacts. The current
  `.gitignore` is deliberately small and is not a comprehensive secret/data boundary.

## Git and CI Operations

- Keep commits focused and use the repository's conventional prefixes (`feat:`, `fix:`, `test:`,
  `docs:`, `chore:`) with an optional scope.
- Do not use the GitHub CLI (`gh`) in this environment. For Actions status or failure diagnosis,
  use the GitHub Actions MCP tools first and the `gh-actions-mcp tool ...` CLI only if MCP tooling
  is unavailable. Do not use `make ci-fail`; the checked-in helper relies on `gh`.
- After successful verification, commit and push all worktree changes by default unless the user
  explicitly asks not to publish. Never overwrite or discard unrelated user changes.
