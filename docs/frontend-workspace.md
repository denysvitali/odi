# Document workspace

The overview brings together recent documents, favorites, saved searches, and deadlines extracted
from the archive. Counts and documents come from ODI's API; failed requests display an unavailable
state and a retry action. A missing extracted deadline does not imply a document has no obligations.
Open the source document before acting on an extracted date.

## Finding documents

Use the overview search field for full-text search, including quoted phrases and AND / OR / NOT.
Filters can search without a text query. Category shortcuts search indexed document types; documents
without classification do not appear in those category results. Search text and filters live in the
URL, so browser back/forward and bookmarked links restore the same search. Links may contain sensitive
search terms; only share them with people who should see those terms. Document access still uses the
configured API authentication.

Choose **Save search**, give it a name, and save it on the device. The overview can reopen or remove
saved searches, or rename them without changing their query and filters. Up to twelve searches are stored in this browser's local storage, including their
filters. They are not synced to other devices, and clearing browser storage removes them. Storage
failures are reported instead of claiming the search was saved.

Mobile search filters use a keyboard-accessible drawer with focus trapping and Escape dismissal.
Selected filters stay available when facet suggestions are missing. Barcode presence supports both
with-barcode and without-barcode searches.

## Browsing

Document listings provide list and grid views; the browser remembers the preference. Sorting changes
only the documents currently loaded, not the archive-wide server order. Load more pages to include
more documents. The overview's recent listing starts in list view. Row actions open document details,
star documents, or select them when bulk selection is enabled. Grid cards support keyboard activation
and arrow navigation while the cards have focus.

Favorites load saved document IDs directly, including documents outside the first archive page.
Individual failures leave the remaining favorites available and can be retried. Search results in
list view display sanitized matching snippets. Upcoming deadlines are grouped into the next seven
days, the next thirty days, and later dates.

The desktop sidebar exposes every application route. On phones, the navigation button opens a
keyboard-accessible drawer with focus trapping and Escape dismissal. The header retains search,
upload, and theme controls. Light/dark mode follows the device until manually selected; reduced-motion
preferences apply across the interface. All assets and fonts are local.

## Verification

Run from `frontend/`:

```sh
pnpm exec vitest run --maxWorkers=2
pnpm run lint
pnpm run build
pnpm exec playwright test
```

The browser cases use synthetic API fixtures and run against the built frontend at desktop and phone
sizes. They cover saved search restoration, filter-only requests, sorting, view switching, navigation,
manual theme selection, overflow, and retryable search failures without requiring real documents or
services.
