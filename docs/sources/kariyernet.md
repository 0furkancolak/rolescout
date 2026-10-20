# Source Review - kariyer.net category lists (`kariyernet`)

Reviewed: 2026-10-20
Adapter: `kariyernet` (`api/internal/ingest/kariyernet`)

## Access method
kariyer.net is a large Turkish job site. Its category list pages
(`https://www.kariyer.net/is-ilanlari/<category>`, then `…/<category>-2`, `-3`
and so on) are rendered on the server and public to every visitor. Each card
carries the title, the employer, the city, the work model and the employment
type, and links to the posting (`/is-ilani/<slug>-<id>`). The adapter reads the
cards only. Posting pages are rendered in the browser and carry no structured
data, so there are no descriptions: matching works from the title, employer,
city and type.

One source per category, created by a migration: yazılım, yazılım geliştirici,
yazılım mühendisi, yazılım uzmanı, mobil yazılım geliştirici, bilgisayar
mühendisi, bilgi işlem, veri bilimci, veri analisti, siber güvenlik, UI/UX
tasarımcı. Add more with `admin sources add -adapter kariyernet -config
'{"url":"https://www.kariyer.net/is-ilanlari/<category>"}'`; the address must
be a list under `/is-ilanlari/` on `www.kariyer.net`, and `max_pages` (default
12, about 50 jobs per page) caps a category.

Listings have no company record: each card names its own employer, so the
job's company is that name. The same job on an employer's own board is
merged by the usual dedupe key.

## Terms and limits
- `robots.txt` of kariyer.net allows `/is-ilanlari/*` and lists, among others,
  `/filtre`, `/ozgecmis/*`, `/basvuru-*`, `/gelen-kutusu` and `/ayarlar` as
  disallowed; the crawler checks it for every request and never leaves the list
  pages. No sign-in, no CV or application pages, no API.
- Its terms of use may restrict automated reading of the site; robots.txt is the
  published signal of what automated clients may fetch, and this adapter stays
  within it. An administrator who cannot accept that can disable the adapter
  (`admin adapters disable -adapter kariyernet`).
- The site answers 403 to clients that read fast (seen while testing at about
  one request per second). All lists share one process-wide gate of 3 s between
  requests, so a full sweep of eleven categories takes several minutes; a 403 on
  the first page is retried later, and on a later page the listings read so far
  are stored and the snapshot is marked incomplete, so nothing is closed.
- A list that returns no cards is incomplete too. Jobs are closed only when a
  list was read to its last page.
- Syncs run every 12 hours.

## Other Turkish job sites checked
- **secretcv.com** - robots.txt disallows the paged lists (`/?sf=…`); only a
  first page of mostly retail postings would be readable. Not supported.
- **eleman.net** - robots.txt disallows its listing pages. Not supported.
- **yenibiris.com** - behind a Cloudflare challenge. Not supported.
- **LinkedIn, Indeed** - robots/terms forbid automated reading. Not supported.

## Verification
Unit tests read a trimmed real list page (six cards), paging by the `rel=next`
link, the page limit, a 403 on a later page, a 404/403 on the first page, and
config validation.
