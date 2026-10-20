# Source Review - Hacker News "Who is hiring?" (`hnhiring`)

Reviewed: 2026-10-21
Adapter: `hnhiring` (`api/internal/ingest/hnhiring`)

## Access method
Every month the `whoishiring` account posts "Ask HN: Who is hiring?"; employers
answer with one top-level comment per role, usually starting with a header such as
`Company | Role | Location | Remote | Full-time`. The adapter finds the newest
threads through the HN Search API (`https://hn.algolia.com/api/v1/search_by_date`)
and reads their top-level comments (1000 per request, at most 5 requests per
thread). A comment is a job post only if its first line has at least three
`|`-separated parts; the company is the first part, the role the first part that
looks like a role, the location the first part that looks like a place.
Replies and free-text comments are skipped. Config: `{"threads": "2"}` (1-3,
default 1).

## Terms and limits
- The HN Search API is public and keyless; it has no `robots.txt`. A sync makes
  about 4 requests, twice a day at most.
- Each job links to its comment on news.ycombinator.com.
- Parsing is heuristic: a company that writes its header differently may get a
  wrong title or location. The full comment is kept as the description.
- Only the newest threads are read, so the snapshot is always incomplete and
  nothing is closed.

## Verification
Unit tests cover header parsing, reply skipping and thread selection. A live run on
2026-10-21 returned 232 posts from the October 2026 thread.
