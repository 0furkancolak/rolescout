# Source Review - Breezy HR career site JSON list

Reviewed: 2026-10-20
Adapter: `breezy` (`api/internal/ingest/breezy`)

## Access method
Every Breezy HR career site (`{subdomain}.breezy.hr`) serves its published positions as JSON, the list its own page and embeds are built from. The list carries the title, link, date, type, location, department and sometimes the salary text, but no description; the department and salary stand in as the description.

Endpoint: `GET https://{subdomain}.breezy.hr/json`

Configuration: `{"subdomain": "acme"}`

## Terms and limits
- `robots.txt` disallows only static asset folders (`/css`, `/fonts`, `/stylesheets`, `/javascripts`) and names one SEO bot; `/json` is allowed.
- Listings keep their original URL.
- Requests go through RoleScout's crawler: they identify RoleScout, follow
  `robots.txt` and share the per-vendor rate limit (see
  [ADR 0004](../adr/0004-discovery-and-adapter-review.md)).
- HTML descriptions are converted to plain text before they are stored.

## Verification
On 2026-10-20 the list of `breezy.breezy.hr` (Breezy's demo company, 3 positions) was read from the real network and saved as the test fixture (`testdata/json.json`).
