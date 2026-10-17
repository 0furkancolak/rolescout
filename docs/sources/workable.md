# Source Review - Workable jobs widget API

Reviewed: 2026-10-18
Adapter: `workable` (`api/internal/ingest/workable`)

## Access method
Workable offers a jobs widget so companies can embed their open jobs on their
own sites. Its JSON endpoint is public and read-only.

Endpoint: `GET https://apply.workable.com/api/v1/widget/accounts/{account}?details=true`.

Configuration: `{"account": "acme"}`

## Terms and limits
- `robots.txt` of `apply.workable.com` disallows nothing.
- Jobs keep their `apply.workable.com/j/{shortcode}` URL.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
