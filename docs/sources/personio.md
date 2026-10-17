# Source Review - Personio job feed

Reviewed: 2026-10-18
Adapter: `personio` (`api/internal/ingest/personio`)

## Access method
Personio career pages publish an XML feed of open positions meant for job
boards and company sites.

Endpoint: `GET https://{subdomain}.jobs.personio.{de|com}/xml?language=en`.

Configuration: `{"subdomain": "acme", "tld": "de"}`

## Terms and limits
- `robots.txt` of career pages allows all paths.
- Positions link to `https://{subdomain}.jobs.personio.{tld}/job/{id}`.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
