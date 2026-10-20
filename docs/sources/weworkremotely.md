# Source Review - We Work Remotely (`weworkremotely`)

Reviewed: 2026-10-21
Adapter: `weworkremotely` (`api/internal/ingest/weworkremotely`)

## Access method
We Work Remotely publishes RSS feeds: all jobs at `/remote-jobs.rss` and one feed per
category at `/categories/<category>.rss`. Items carry the title ("Company: Job
title"), region, type, a full HTML description and the job URL. One source per
category is created by a migration (programming, full-stack, front-end, back-end,
devops, design, product, management and finance, customer support, sales and
marketing).

## Terms and limits
- Feeds are the site's published syndication format; `robots.txt` allows them
  (`Allow: /`). Each job keeps its We Work Remotely URL.
- One request per source every 8 hours.
- A feed window: the snapshot is always incomplete and nothing is closed.
- Jobs are stored as remote; the region becomes the location.

## Verification
Unit tests cover the title split and field mapping. A live run on 2026-10-21
returned 25 jobs from the programming category.
