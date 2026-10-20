# Job Sources

RoleScout imports listings through source adapters run by the worker. A
source is one configured feed, such as one company's Greenhouse board.

## Rules
- Only use access methods a source offers for this purpose: documented public
  APIs or feeds. The exception is pages a site serves to every visitor as its
  job list (`jsonld`, `htmlpage`, `kariyernet`): they are read only where
  `robots.txt` allows, politely and capped, and each has its own review. Do
  not bypass authentication, rate limits or robots rules, or use credentials
  that were not issued for this use.
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
| `teamtailor` | Teamtailor career site RSS feed (`/jobs.rss`) | [teamtailor.md](teamtailor.md) |
| `breezy` | Breezy HR career site JSON list (`/json`) | [breezy.md](breezy.md) |
| `rippling` | Rippling ATS board API (list only) | [rippling.md](rippling.md) |
| `gem` | Gem job board API (Greenhouse-compatible) | [gem.md](gem.md) |
| `personio` | Personio XML job feed | [personio.md](personio.md) |
| `bamboohr` | BambooHR careers list | [bamboohr.md](bamboohr.md) |
| `jsonld` | schema.org JobPosting data on company career sites | [jsonld.md](jsonld.md) |
| `htmlpage` | job cards printed in the HTML of a company's own career page | [htmlpage.md](htmlpage.md) |
| `kariyernet` | kariyer.net public category lists (cards only), paced and capped | [kariyernet.md](kariyernet.md) |
| `workday` | Workday career site endpoint, **off until reviewed** | [workday.md](workday.md) |
| `remoteok` | Remote OK public JSON API (many employers) | [remoteok.md](remoteok.md) |
| `arbeitnow` | Arbeitnow free job board API (Germany and neighbours) | [arbeitnow.md](arbeitnow.md) |
| `himalayas` | Himalayas public remote-jobs API | [himalayas.md](himalayas.md) |
| `jobicy` | Jobicy public remote-jobs API, one source per industry | [jobicy.md](jobicy.md) |
| `themuse` | The Muse public jobs API, one source per category | [themuse.md](themuse.md) |
| `weworkremotely` | We Work Remotely RSS feeds, one source per category | [weworkremotely.md](weworkremotely.md) |
| `workingnomads` | Working Nomads public job feed | [workingnomads.md](workingnomads.md) |
| `hnhiring` | Posts of the monthly "Ask HN: Who is hiring?" thread | [hnhiring.md](hnhiring.md) |

Job sites such as these list postings of many employers and belong to no company;
each listing names its own employer. They are windows onto the newest postings, so
their snapshots are always marked incomplete and never close jobs.

### Not added
| Site | Reason |
| --- | --- |
| Remotive | The API is documented, but `robots.txt` disallows `/api/*` for every client, and its terms allow only a few requests a day. |
| kariyer.net (more categories) | Pages are behind a PerimeterX captcha (403 to automated clients from the server and from a laptop alike). The `kariyernet` adapter stays, but more categories were not added: they could not be verified and bypassing bot detection is out of bounds. |
| eleman.net | `robots.txt` disallows the listing queries (`?t=`, `?tip=`, `?ilan_id=` ...). |
| secretcv.com | `robots.txt` disallows the paged lists and search. |
| yenibiris.com | Cloudflare challenge, also for `robots.txt`. |
| LinkedIn, Indeed, Glassdoor, Monster, ZipRecruiter, StepStone, Totaljobs | Terms or `robots.txt` forbid automated reading, or the site answers 403/Access Denied. |
| JustJoin.it, NoFluffJobs, Landing.jobs | Their `robots.txt` disallows `/api/`, the only structured access. |
| Jobspresso | `robots.txt` disallows every URL with a query string, which includes its feed. |
| Arbeitsagentur Jobbörse, USAJobs, Adzuna, Jooble, Careerjet, Reed | An API key or registration is needed. |
| Dice, Wellfound, Otta, jobs.ch, İŞKUR e-Şube, kamuilan | No public feed or API; the lists are rendered in the browser, behind a login, or behind form postbacks. |

SmartRecruiters is not supported: the robots.txt of its API host disallows all
crawlers except LinkedIn's. The other systems that were looked at and left out
(Pinpoint, Comeet, Join, Jobvite, Freshteam, HiBob, Homerun) are explained in
[more-ats.md](more-ats.md).

## Where companies are discovered
Discovery runs before detection and collection and adds companies to the
catalog. Sources, in order: the shipped seed lists, the name probe, **stock
exchange listings** (Nasdaq Trader: Nasdaq, NYSE and other US exchanges),
**sectors** (the AI names companies per industry and region; only when AI is
on), a probe of catalog names without a board, Hacker News "Who is hiring?"
and community Markdown lists. **DBpedia** lists, per country (the run's focus countries first, then TR, US, GB,
DE, NL and FR), the companies that have a website on that country's domain
(`.tr`, `.de`, `.uk`, …) or, for the US, are placed there; their sites go to the
detector, which looks for a board, JSON-LD or job cards on the page and then
tries the usual career addresses (`/kariyer`, `/careers`, `/is-ilanlari`, … and
career pages named in `sitemap.xml`). Its public SPARQL endpoint is read 2 000
rows at a time, five requests per run at most; a finished country is read again
on the normal discovery schedule. Wikidata's endpoint would give more companies
but its `robots.txt` disallows `/sparql`, so it is not used. OpenStreetMap's
Overpass service was tried and left out: a country query takes over a minute.
Exchange and sector sources produce only names; a
name is added only when its job board exists on Lever, Greenhouse, Ashby,
Workable or Recruitee, so a wrong name costs a few requests and nothing else.
They are read in batches (150 names, two sector/region pairs per run).
