# Source Review - schema.org JobPosting (JSON-LD)

Reviewed: 2026-10-18
Adapter: `jsonld` (`api/internal/ingest/jsonld`)

## Access method
Many companies build their own career sites. To appear in search engines'
job results they embed each posting as schema.org `JobPosting` structured
data in a `<script type="application/ld+json">` block. This data is published
for machines to read.

The adapter starts at the configured career page, reads the JSON-LD on it and
follows links that stay on the same host under the same path (for example
`/careers/...`), breadth first, through every page it finds (set `JSONLD_MAX_PAGES` to cap it; default 0 = no cap).
Objects, arrays and `@graph` wrappers are understood; postings whose
`validThrough` has passed are skipped. Non-HTML responses, broken scripts and
missing linked pages are skipped; only a failure of the start page fails the
sync.

Configuration: `{"url": "https://acme.example/careers", "company": "Acme"}`

## Terms and limits
- Every page request checks the site's `robots.txt` and honours
  `Crawl-delay`; company sites get one request at a time, at least
  `CRAWL_HOST_INTERVAL` (1 s) apart.
- Only pages under the career path are read; the rest of the site is not.
- Postings keep their `url` (or the page they were found on).
- Descriptions are converted to plain text before they are stored.

## Verification
Unit tests cover single objects, arrays, `@graph`, `@type` arrays, malformed
scripts, expired postings, link filtering, the page limit and non-HTML
responses. An integration test checks that the same job from Greenhouse and
from a JSON-LD page is stored once.
