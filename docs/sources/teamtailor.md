# Source Review - Teamtailor career site RSS feed

Reviewed: 2026-10-20
Adapter: `teamtailor` (`api/internal/ingest/teamtailor`)

## Access method
Every Teamtailor career site publishes its open jobs as an RSS feed, meant for job aggregators and feed readers. The items carry the title, the HTML description, the publication date, the link, remote status and the locations.

Endpoint: `GET https://{subdomain}.teamtailor.com/jobs.rss`

Configuration: `{"subdomain": "acme"}`

## Terms and limits
- `robots.txt` of career sites disallows only `/app/`, `/messages/`, `/messenger/` and `/facebook/tab/` for ordinary crawlers; the feed is allowed.
- Companies that serve Teamtailor on their own domain are not covered; they are found by the `htmlpage` or `jsonld` adapters instead.
- Listings keep their original URL.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-20 the feed of `career.teamtailor.com` (Teamtailor's own board, 14 jobs) was read from the real network; two items, with the descriptions shortened, are the test fixture (`testdata/jobs.rss`).
