-- Source adapters, permission review and ingestion run history (Phase 6).

ALTER TABLE job_sources
    ADD COLUMN adapter TEXT NOT NULL DEFAULT 'manual' CHECK (adapter ~ '^[a-z0-9_]+$'),
    ADD COLUMN config JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(config) = 'object'),
    ADD COLUMN sync_interval INTERVAL NOT NULL DEFAULT '6 hours' CHECK (sync_interval >= INTERVAL '5 minutes'),
    ADD COLUMN reviewed_at TIMESTAMPTZ,
    ADD COLUMN review_note TEXT,
    -- A source can only be enabled after its access method and terms were
    -- reviewed and the outcome was written down (docs/PLAN.md, section 7).
    ADD CONSTRAINT job_sources_enabled_requires_review
        CHECK (NOT enabled OR (reviewed_at IS NOT NULL AND btrim(coalesce(review_note, '')) <> ''));

CREATE TABLE ingestion_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES job_sources (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    fetched_count INTEGER NOT NULL DEFAULT 0 CHECK (fetched_count >= 0),
    upserted_count INTEGER NOT NULL DEFAULT 0 CHECK (upserted_count >= 0),
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    error TEXT,
    CHECK ((status = 'running') = (finished_at IS NULL)),
    CHECK (status <> 'failed' OR error IS NOT NULL)
);
CREATE INDEX ingestion_runs_source_started_idx ON ingestion_runs (source_id, started_at DESC);
-- At most one run per source can be in progress.
CREATE UNIQUE INDEX ingestion_runs_one_running_idx ON ingestion_runs (source_id) WHERE status = 'running';
