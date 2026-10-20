# Source Review - Gem job board API

Reviewed: 2026-10-20
Adapter: `gem` (`api/internal/ingest/gem`)

## Access method
Gem hosts job boards at `jobs.gem.com/{board}` and publishes them through a Greenhouse-compatible Job Board API. The list includes the HTML description, employment type, location type and dates.

Endpoint: `GET https://api.gem.com/job_board/v0/{board}/job_posts/`

Configuration: `{"board": "acme"}`

## Terms and limits
- `api.gem.com` answers `/robots.txt` with 403. RFC 9309 treats an unreadable-by-client-error robots file as "no rules", and RoleScout's crawler does the same; the API is Gem's documented public job board API and needs no credentials.
- Listings keep their original URL.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-20 the board `gem` (4 jobs) was read from the real network; two jobs, shortened, are the test fixture (`testdata/jobs.json`).
