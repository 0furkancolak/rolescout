# Source Review - Greenhouse Job Board API

Reviewed: 2026-10-17
Adapter: `greenhouse` (`api/internal/ingest/greenhouse`)

## Access method
Greenhouse hosts job boards for employers and publishes them through the Job
Board API (https://docs.greenhouse.io/job-board.html). The documentation
states that job board data is publicly available and that no authentication
is required for any GET endpoint. The API exists so that employers' open jobs
can be shown outside Greenhouse.

The adapter uses two read-only endpoints per sync:
- `GET https://boards-api.greenhouse.io/v1/boards/{board_token}` for the
  board's company name (skipped when `company` is configured), and
- `GET https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs?content=true`
  for the jobs.

It never calls the application submission endpoint and holds no credentials.

## Terms and limits
- The documentation lists no rate limit. The adapter makes at most two
  requests per board per sync, the default interval is 6 hours (minimum 5
  minutes), and retries are limited to three attempts with backoff.
- Requests identify RoleScout with a `User-Agent` that links to the project.
- Each job keeps its `absolute_url`, which links back to the employer's
  posting where candidates apply.
- Job content is HTML-escaped HTML; it is converted to plain text before it is
  stored, so no source markup is rendered by RoleScout.

## Operator checklist for each board
Before enabling a `greenhouse` source, confirm that:
1. the board is the employer's public job board (it is reachable without
   signing in at `https://boards.greenhouse.io/{board_token}`), and
2. nothing on the employer's careers site restricts reuse of the listings.

Record the result with `admin sources review -key ... -note "..."`.

## Verification
On 2026-10-17 the GitLab board (`board_token=gitlab`) was synced on a local
installation through `admin sources sync` and then by the worker's schedule.
Each run fetched 232 listings and stored them as 230 jobs (two postings shared
company, title and location and were deduplicated); repeated runs updated the
same rows. Enabling the source before its review was rejected. The local data
was removed afterwards.
