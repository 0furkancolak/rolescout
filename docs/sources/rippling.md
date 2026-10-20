# Source Review - Rippling ATS board API

Reviewed: 2026-10-20
Adapter: `rippling` (`api/internal/ingest/rippling`)

## Access method
Rippling's job boards (`ats.rippling.com/{board}/jobs`) are built on a public JSON API. The list carries the title, link, department and work location, but no description (it is in a per-job call that this adapter does not make, to keep the request count at one per board).

Endpoint: `GET https://api.rippling.com/platform/api/ats/v1/board/{board}/jobs`

Configuration: `{"board": "acme"}`

## Terms and limits
- `api.rippling.com` has no `robots.txt` (404), so nothing is disallowed; the board pages' own `robots.txt` (`ats.rippling.com`) disallows only `/internal/`.
- The API takes no credentials.
- Listings keep their original URL.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-20 the board `rippling` (655 jobs) was read from the real network; three jobs are the test fixture (`testdata/jobs.json`).
