# Source Review - Workday career sites (not reviewed by default)

Reviewed: 2026-10-18
Adapter: `workday` (`api/internal/ingest/workday`)

## Access method
Workday career sites (`{tenant}.wd{n}.myworkdayjobs.com/{site}`) load jobs
from `POST /wday/cxs/{tenant}/{site}/jobs`. This endpoint is **not a
documented public API**. RoleScout includes the adapter but ships it without
a review, so no Workday source can be enabled until an administrator reviews
the terms of the employers concerned and enables the adapter type:

```bash
docker compose run --rm admin adapters review -adapter workday -note "..."
docker compose run --rm admin adapters enable -adapter workday
```

The list has no descriptions; jobs link to the career site. Up to 100 pages
of 20 jobs are read per sync.

Configuration: `{"host": "acme.wd5.myworkdayjobs.com", "tenant": "acme", "site": "External"}`

## Terms and limits
- Career sites' `robots.txt` usually allows the site path; the crawler checks
  it for the API path too.
- All Workday tenants share one vendor rate limit.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
