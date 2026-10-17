# Job Sources

RoleScout imports listings through source adapters run by the worker. A
source is one configured feed, such as one company's Greenhouse board.

## Rules
- Only use access methods a source offers for this purpose: documented public
  APIs or feeds. Do not scrape pages, bypass authentication, rate limits or
  robots rules, or use credentials that were not issued for this use.
- Every adapter has a review in this folder describing the access method, the
  terms that apply and the limits the adapter respects.
- Every configured source starts disabled. An operator must record a review
  (`admin sources review`) before it can be enabled; the database rejects
  enabling an unreviewed source.
- Keep original URLs. The UI links every job to its source listings.

## Operating sources
The `admin` command manages sources. With Docker Compose:

```bash
docker compose run --rm admin sources add -key greenhouse-acme -name "Acme" \
  -adapter greenhouse -config '{"board_token":"acme"}' \
  -base-url https://boards.greenhouse.io/acme -interval 6h
docker compose run --rm admin sources review -key greenhouse-acme \
  -note "Greenhouse public Job Board API; reviewed per docs/sources/greenhouse.md; Acme publishes this board publicly."
docker compose run --rm admin sources sync -key greenhouse-acme   # one run now, to test
docker compose run --rm admin sources enable -key greenhouse-acme # scheduled syncs
docker compose run --rm admin sources list
docker compose run --rm admin sources runs -key greenhouse-acme
```

## How ingestion works
- The worker checks every `WORKER_TICK` (default `1m`) for enabled sources
  whose last run started at least one sync interval ago (default `6h`, at least
  `5m`). After a failed run the source is retried after a quarter of its
  interval, but not sooner than five minutes.
- Each sync is recorded in `ingestion_runs` with its status, attempts, counts
  and error. Only one run per source can be in progress; a run left `running`
  for over an hour (for example after a crash) is marked failed.
- Temporary fetch errors (timeouts, 429, 5xx) are retried up to three times
  with exponential backoff. Other errors fail the run immediately.
- Listings that fail validation or storage are skipped and counted; the run
  still succeeds if at least one listing was stored, and its error field
  summarizes what was skipped.
- Storing is idempotent: a listing is identified by its source and the
  source's external ID, and a job by its dedupe key. See
  [ADR 0003](../adr/0003-source-ingestion.md).
- The worker logs JSON lines (`source sync started`/`finished`/`failed`) with
  the source key, run ID, counts and duration.

## Adapters
| Adapter | Access method | Review |
| --- | --- | --- |
| `greenhouse` | Greenhouse public Job Board API | [greenhouse.md](greenhouse.md) |
