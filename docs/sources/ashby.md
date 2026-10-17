# Source Review - Ashby Job Postings API

Reviewed: 2026-10-18
Adapter: `ashby` (`api/internal/ingest/ashby`)

## Access method
Ashby's public Job Postings API
(https://developers.ashbyhq.com/docs/public-job-posting-api) returns a
company's published jobs without authentication, for use on career sites.

Endpoint: `GET https://api.ashbyhq.com/posting-api/job-board/{board}?includeCompensation=false`.
Unlisted jobs (`isListed: false`) are skipped.

Configuration: `{"board": "acme", "company": "Acme"}`

## Terms and limits
- `robots.txt` on the API host answers 401, which RFC 9309 treats as no
  restrictions; requests still go through the vendor rate limit.
- Jobs keep their `jobUrl` on jobs.ashbyhq.com.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
