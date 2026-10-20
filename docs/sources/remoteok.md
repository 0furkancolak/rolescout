# Source Review - Remote OK (`remoteok`)

Reviewed: 2026-10-21
Adapter: `remoteok` (`api/internal/ingest/remoteok`)

## Access method
Remote OK publishes its newest remote jobs as JSON at `https://remoteok.com/api`
(about 100 jobs, no key). The first element of the array is a legal notice; the
adapter skips it. Each job carries the title, company, description, location, tags
and the Remote OK URL.

## Terms and limits
- The API's terms ask for a link back to the job on Remote OK and a mention of
  Remote OK as the source. Every job keeps its Remote OK URL (`url`) as its
  listing link, and the interface links each job to its source listings.
- The Remote OK logo is not used.
- `robots.txt` allows `/api` (it disallows only the AJAX `get_jobs` endpoints,
  and sets `Crawl-delay: 1`, which the crawler honours). One request per sync,
  every 6 hours.
- The feed is a window onto the newest jobs: the snapshot is always marked
  incomplete, so jobs that left it are not closed.
- Every job is stored as remote.

## Verification
Unit tests cover the legal-notice element, the field mapping and the error
mapping. A live run on 2026-10-21 returned 99 jobs.
