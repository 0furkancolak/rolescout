# ADR 0003 - Source Ingestion

Status: Accepted (review per source superseded by [ADR 0004](0004-discovery-and-adapter-review.md))
Date: 2026-10-17

## Context
Phase 6 imports listings from external sources. Sources differ in format and
reliability, may only be used within their terms, and can fail temporarily.
Imports must not create duplicates, and failures must be diagnosable.

## Decisions

### Adapter contract
An adapter (`ingest.Adapter`) validates its JSON configuration and fetches a
full snapshot of a source's current listings as `RawListing` values. Adapters
only translate formats; validation, normalization, retries and storage are
shared in `ingest.Runner`. Adapters mark temporary failures with
`ingest.Retryable`. Adapters are registered by name in
`ingest/adapters`; a source row stores the adapter name and configuration.

### Normalization
`ingest.Normalize` validates required fields (external ID, title, company,
absolute http(s) URL), trims lengths, converts HTML descriptions to plain text
and infers work arrangement, employment type, seniority and technologies from
the title and description with word-boundary keyword rules. Unknown values
stay `unknown` rather than guessed. Ambiguous technology names such as "go"
are only matched in the title.

### Deduplication
- The same listing seen again is matched by `(source_id, external_id)`.
- The same job from different sources is matched by a dedupe key: SHA-256 of
  the normalized company, title and location, prefixed `v1:`. Normalization
  lower-cases and keeps only letters and digits. The prefix lets a future
  algorithm coexist with stored keys.
- Upserts update the job and listing in place; `first_seen_at` only moves
  earlier and `last_seen_at` only later.

### Scheduling and failure handling
- The worker polls for due sources and syncs them one at a time. Each sync is
  an `ingestion_runs` row; a partial unique index allows only one running run
  per source, so several workers cannot sync the same source concurrently.
- Retryable fetch errors are retried three times with exponential backoff.
  A failed run makes the source due again after a quarter of its interval.
- Runs stuck in `running` for an hour are marked failed on the next tick.
- Per-listing failures are counted and summarized without failing the run,
  unless no listing could be stored.

### Permissions
Enabling a source requires `reviewed_at` and a non-empty `review_note`,
enforced by a CHECK constraint. Each adapter has a review in `docs/sources/`.

## Consequences
- Listings that disappear from a source are not removed; `last_seen_at`
  shows freshness. Marking closed listings is left for later.
- Keyword inference is simple and transparent; it can be refined without a
  schema change, but changing the dedupe algorithm needs a new key version.
