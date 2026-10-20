# Source Review - Himalayas (`himalayas`)

Reviewed: 2026-10-21
Adapter: `himalayas` (`api/internal/ingest/himalayas`)

## Access method
Himalayas publishes its remote jobs as JSON at `https://himalayas.app/jobs/api`
(20 jobs per page, newest first, cursor paging with `nextCursor`). No key. The
adapter reads up to `max_pages` pages (default 15, so 300 jobs).

## Terms and limits
- The listing link is the plain Himalayas job page (`guid`), so Himalayas gets the
  visit; the tracked `applicationLink` is not used.
- `robots.txt` allows `/jobs/api` (it disallows paged HTML lists, which the adapter
  does not read). Paged requests go through the crawler's per-host pacing; a sync
  is every 6 hours.
- Newest pages only: the snapshot is always incomplete and nothing is closed.
- Jobs are stored as remote; location restrictions become the location.

## Verification
Unit tests cover cursor paging and field mapping. A live run on 2026-10-21
returned 40 jobs for two pages.
