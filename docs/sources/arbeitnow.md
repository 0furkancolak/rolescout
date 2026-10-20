# Source Review - Arbeitnow (`arbeitnow`)

Reviewed: 2026-10-21
Adapter: `arbeitnow` (`api/internal/ingest/arbeitnow`)

## Access method
Arbeitnow, a job board focused on Germany and its neighbours (visa-sponsorship and
English-language jobs included), offers a free public API at
`https://www.arbeitnow.com/api/job-board-api`: 300 jobs per page, newest first,
paged with `?page=`. No key. The adapter reads up to `max_pages` pages (default 3).

## Terms and limits
- The API response states: free public API, "please do not abuse", linking back
  appreciated. Every job keeps its Arbeitnow URL as its listing link. One request
  per page, every 6 hours, through the polite crawler.
- `robots.txt` has no relevant disallow.
- Only the newest pages are read, so the snapshot is always incomplete and jobs
  that left them are not closed.

## Verification
Unit tests cover paging, the page limit and field mapping. A live run on
2026-10-21 returned 300 jobs for one page.
