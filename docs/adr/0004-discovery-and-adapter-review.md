# ADR 0004 - Company Discovery and Adapter-Type Review

Status: Accepted
Date: 2026-10-18

## Context
Users want RoleScout to find jobs on companies' own career pages by itself,
for free and at scale (thousands of companies). Under ADR 0003, every source
needed its own review before it could be enabled. With thousands of
discovered companies that rule blocks the product, yet almost all of them use
a handful of public job board APIs whose terms are the same for every company.
See the [design](../superpowers/specs/2026-10-17-career-page-research-design.md).

## Decisions

### Company catalog
- `companies` is global: career pages are public information. Each company
  has a careers URL, the detected job board system (`ats`, `ats_id`), where it
  was found (`commoncrawl`, `seed`, `link`, `manual`), a status (`pending`,
  `active`, `empty`, `unreachable`, `blocked_by_robots`, `disabled`) and
  failure backoff (1 hour doubled per failure, at most 7 days).
- A company with a detected board gets one job source (`job_sources.company_id`),
  so syncing, run history and deduplication reuse ADR 0003 unchanged.
- Workspaces can block companies (`workspace_company_blocks`); blocks are
  private to the workspace. A manually added company joins the global catalog
  without recording who added it.

### Review moves to the adapter type
- `source_adapter_reviews` records a review per adapter type. A source can be
  enabled when it has its own review or its adapter type is reviewed and
  enabled. A trigger enforces this and raises the old constraint name, so
  existing error handling keeps working.
- The migration records reviews for the public job board APIs (Greenhouse,
  Lever, Ashby, Workable, SmartRecruiters, Recruitee, Personio, BambooHR) and
  for schema.org `JobPosting` data (`jsonld`). Each has a page in
  `docs/sources/`.
- `workday` uses an undocumented endpoint behind Workday career sites, so it
  ships without a review: an administrator must review and enable it.
- The first account is the installation administrator (`users.is_admin`).
  Only administrators can review adapter types and disable companies for
  everyone.

### Polite crawling
All outbound requests go through `internal/crawl`:
- User-Agent `RoleScout/<version> (+<CRAWL_USER_AGENT_URL>)`.
- `robots.txt` is read per origin, cached for 24 hours and applied to every
  hop of a redirect; a 5xx robots.txt disallows the host until it is read
  again (RFC 9309). A robots.txt that cannot be reached is not cached.
- Ordinary hosts get one request at a time, at least `CRAWL_HOST_INTERVAL`
  (1 s) apart. Job board API hosts that serve many companies get
  `CRAWL_VENDOR_CONCURRENCY` (4) parallel requests at least 250 ms apart.
- Responses are capped at 32 MiB; 429 and 5xx are temporary and retried with
  backoff, honouring `Retry-After`.
- Conditional requests (`ETag`, `Last-Modified`) are used for pages read
  repeatedly (robots.txt, Common Crawl metadata, JSON-LD job pages). Board
  APIs are always read in full, because closing jobs that disappeared needs
  the whole snapshot.

### Retention
Closed jobs that nobody saved or applied to are deleted after
`CLOSED_JOB_RETENTION` (90 days by default, `0` keeps them).

## Consequences
- Discovered companies sync without a manual step, within reviewed terms.
- Adding an adapter type means writing its review page and a migration row,
  or reviewing it at runtime with `admin adapters review`.
- Crawl speed is bounded by vendor limits: about 20 000 boards per hour or
  two at the default settings.
