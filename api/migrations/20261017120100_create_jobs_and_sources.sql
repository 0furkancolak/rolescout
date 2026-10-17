CREATE TABLE job_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL UNIQUE CHECK (key ~ '^[a-z0-9][a-z0-9_-]*$'),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    base_url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TRIGGER job_sources_set_updated_at BEFORE UPDATE ON job_sources
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dedupe_key TEXT NOT NULL UNIQUE CHECK (btrim(dedupe_key) <> ''),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    company_name TEXT NOT NULL CHECK (btrim(company_name) <> ''),
    location TEXT,
    work_arrangement TEXT NOT NULL DEFAULT 'unknown'
        CHECK (work_arrangement IN ('onsite', 'hybrid', 'remote', 'unknown')),
    employment_type TEXT NOT NULL DEFAULT 'unknown'
        CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'internship', 'temporary', 'unknown')),
    seniority TEXT NOT NULL DEFAULT 'unknown'
        CHECK (seniority IN ('intern', 'junior', 'mid', 'senior', 'lead', 'unknown')),
    technologies TEXT[] NOT NULL DEFAULT '{}',
    description TEXT NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (last_seen_at >= first_seen_at)
);
CREATE INDEX jobs_last_seen_idx ON jobs (last_seen_at DESC, id);
CREATE INDEX jobs_technologies_idx ON jobs USING GIN (technologies);
CREATE TRIGGER jobs_set_updated_at BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();

CREATE TABLE job_source_listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES job_sources (id) ON DELETE RESTRICT,
    external_id TEXT NOT NULL CHECK (btrim(external_id) <> ''),
    url TEXT NOT NULL CHECK (url ~ '^https?://'),
    source_posted_at TIMESTAMPTZ,
    source_updated_at TIMESTAMPTZ,
    first_ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, external_id)
);
CREATE INDEX job_source_listings_job_idx ON job_source_listings (job_id);
