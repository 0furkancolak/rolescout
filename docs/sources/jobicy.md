# Source Review - Jobicy (`jobicy`)

Reviewed: 2026-10-21
Adapter: `jobicy` (`api/internal/ingest/jobicy`)

## Access method
Jobicy's public API (`https://jobicy.com/api/v2/remote-jobs`, no key) returns up to
100 of the newest remote jobs, optionally filtered by `industry`, `geo` and `tag`.
One source per industry is created by a migration (engineering, data-science,
design-multimedia, business, marketing, supporting, admin-support, copywriting,
plus one unfiltered).

## Terms and limits
- The API's notice asks to credit Jobicy with a direct link to the source and to
  send applicants to the original job URL. Each job keeps its Jobicy URL, which is
  where the job's own apply link lives.
- `robots.txt` does not disallow `/api/`. One request per source, every 12 hours;
  the API is documented as rate limited, so sources are not synced more often.
- A feed window: the snapshot is always incomplete and nothing is closed.

## Verification
Unit tests cover the query, field mapping and errors. A live run on 2026-10-21
returned 100 jobs for the engineering industry.
