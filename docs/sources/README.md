# Job Sources

RoleScout imports listings through source adapters run by the worker. A
source is one configured feed, such as one company's Greenhouse board.

## Rules
- Only use access methods a source offers for this purpose: documented public
  APIs or feeds. Do not scrape pages, bypass authentication, rate limits or
  robots rules, or use credentials that were not issued for this use.
- Every adapter has a review in this folder describing the access method, the
  terms that apply and the limits the adapter respects.
- A source can only be enabled when its adapter type has an enabled review
  (see below) or the source has its own review (`admin sources review`); the
  database rejects anything else. Sources created by career page research
  for discovered companies are enabled through their adapter type's review.
  See [ADR 0004](../adr/0004-discovery-and-adapter-review.md).
- All requests go through RoleScout's polite crawler: it identifies itself,
  follows `robots.txt` and limits requests per host.
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

## Adapter-type reviews
```bash
docker compose run --rm admin adapters list
docker compose run --rm admin adapters review -adapter workday -note "what was checked, link to terms"
docker compose run --rm admin adapters enable -adapter workday
docker compose run --rm admin adapters disable -adapter workday
```

Companies can also be added by their career page URL; the worker detects the
job board behind it:

```bash
docker compose run --rm admin companies add -url https://acme.example/careers -name Acme
docker compose run --rm admin companies list -status pending
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
| `lever` | Lever Postings API | [lever.md](lever.md) |
| `ashby` | Ashby Job Postings API | [ashby.md](ashby.md) |
| `workable` | Workable jobs widget API | [workable.md](workable.md) |
| `recruitee` | Recruitee careers site API | [recruitee.md](recruitee.md) |
| `personio` | Personio XML job feed | [personio.md](personio.md) |
| `bamboohr` | BambooHR careers list | [bamboohr.md](bamboohr.md) |
| `jsonld` | schema.org JobPosting data on company career sites | [jsonld.md](jsonld.md) |
| `workday` | Workday career site endpoint, **off until reviewed** | [workday.md](workday.md) |

SmartRecruiters is not supported: the robots.txt of its API host disallows all
crawlers except LinkedIn's.
