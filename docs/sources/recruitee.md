# Source Review - Recruitee careers site API

Reviewed: 2026-10-18
Adapter: `recruitee` (`api/internal/ingest/recruitee`)

## Access method
Every Recruitee customer has a careers site at `{subdomain}.recruitee.com`
whose public API lists published offers for the site and for embedding.

Endpoint: `GET https://{subdomain}.recruitee.com/api/offers/`. Only offers with
status `published` are kept.

Configuration: `{"subdomain": "acme"}`

## Terms and limits
- `robots.txt` of careers sites disallows `/v/` only.
- Offers keep their `careers_url`.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
