# Source Review - The Muse (`themuse`)

Reviewed: 2026-10-21
Adapter: `themuse` (`api/internal/ingest/themuse`)

## Access method
The Muse's public jobs API (`https://www.themuse.com/api/public/jobs`) works without
a key at a limited rate. It pages 20 jobs at a time and filters by `category`,
`level` and `location`. One source per category is created by a migration
(software engineering, data and analytics, computer and IT, design and UX, product
management), each restricted to "Flexible / Remote" and read up to `max_pages` (10)
pages. The Muse lists mostly US employers, so it is a small part of the coverage
for other countries.

## Terms and limits
- Each job links to its The Muse landing page (`refs.landing_page`).
- `robots.txt` does not disallow `/api/public/`. Pages are fetched through the
  polite crawler; syncs are every 12 hours, far below the keyless limit.
- The API does not order by date, so a read is a sample: the snapshot is always
  incomplete and nothing is closed.

## Verification
Unit tests cover paging, filters and field mapping. A live run on 2026-10-21
returned 40 jobs for two pages.
