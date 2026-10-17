# Source Review - Lever Postings API

Reviewed: 2026-10-18
Adapter: `lever` (`api/internal/ingest/lever`)

## Access method
Lever publishes each customer's open postings through the Postings API
(https://github.com/lever/postings-api). It is read-only, needs no key, and
exists so that companies can list their jobs on their own sites.

Endpoint: `GET https://api.lever.co/v0/postings/{site}?mode=json`
(`api.eu.lever.co` with `"region": "eu"`). One request per company per sync.

Configuration: `{"site": "acme", "region": "eu", "company": "Acme"}`

## Terms and limits
- `robots.txt` of `api.lever.co` allows all paths and asks for `Crawl-delay: 1`;
  the crawler honours it, so Lever is read one request per second.
- Postings keep their `hostedUrl`, the company's posting where candidates apply.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-18 one real response was saved, trimmed to a few jobs and used as
the adapter's test fixture (`testdata/`).
