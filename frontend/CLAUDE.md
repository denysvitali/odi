# CLAUDE.md — frontend

React 19 + Mantine 9 + TypeScript SPA, built with Vite, tested with Vitest (jsdom) and Playwright.

## Commands (from `frontend/`)

```bash
pnpm dev                 # Vite dev server
pnpm build               # tsc + vite build
pnpm lint                # eslint
pnpm test                # vitest; single file: pnpm test -- src/path/file.spec.ts
pnpm test:e2e            # playwright (desktop + mobile), against `vite preview` of dist/
```

## Layout

- `src/main.tsx` loads `public/settings.json` into `window._settings` before mounting (`lib/config.ts` validates it).
- `src/router.tsx` — routes; pages in `src/pages/` are lazy except `SearchPage`.
- `src/api/client.ts` — the only REST boundary (bearer token, retries). `src/hooks/queries.ts` wraps it in React Query; components never call `fetch` directly.
- `src/stores/` — device-local state (favorites, tags, recent/saved searches, view) on a tiny `useSyncExternalStore` + localStorage helper.
- `src/components/documents/DocumentDrawer.tsx` — the document view, opened from anywhere via the `?doc=<id>` URL param (`hooks/useDocumentDrawer.ts`).
- Search state lives in the URL (`lib/archiveSearch.ts`).

## Conventions

- Use Mantine components/props and Tabler icons; avoid custom CSS (CSS modules only when needed).
- Paged lists use OpenSearch scroll cursors: keep `gcTime: 0` on infinite queries (expired cursors must never be reused).
- Render OCR/highlight text as React text (`Highlighted`), never as HTML.
- Specs live in `__tests__/` next to the code; `src/test/render.tsx` provides app providers.
