# Source Review - BambooHR careers list

Reviewed: 2026-10-18
Adapter: `bamboohr` (`api/internal/ingest/bamboohr`)

## Access method
BambooHR career pages at `{subdomain}.bamboohr.com/careers` load their job
list from a public JSON endpoint.

Endpoint: `GET https://{subdomain}.bamboohr.com/careers/list`. The list has no
descriptions; jobs link to `/careers/{id}`. A subdomain without a career page
redirects to bamboohr.com and the sync fails.

Configuration: `{"subdomain": "acme", "company": "Acme"}`

## Terms and limits
- `robots.txt` disallows only the old embed scripts.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
