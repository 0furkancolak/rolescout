# Source Review - Working Nomads (`workingnomads`)

Reviewed: 2026-10-21
Adapter: `workingnomads` (`api/internal/ingest/workingnomads`)

## Access method
Working Nomads publishes its current remote jobs as JSON at
`https://www.workingnomads.com/api/exposed_jobs/` (the endpoint's name says it is
meant to be exposed; no key). Items carry the title, company, category, tags,
location, publication time and an HTML description. The job link is the site's
redirect (`/job/go/<id>/`), which is used as the listing URL and as the external
ID.

## Terms and limits
- `robots.txt` allows everything. One request every 6 hours.
- A feed window: the snapshot is always incomplete and nothing is closed.
- Jobs are stored as remote.

## Verification
Unit tests cover field mapping and incomplete items. A live run on 2026-10-21
returned 45 jobs.
