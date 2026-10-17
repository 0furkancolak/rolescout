-- Research profiles, runs with stage progress and logs, and scored matches.

CREATE TABLE research_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    title_keywords TEXT[] NOT NULL DEFAULT '{}',
    keywords TEXT[] NOT NULL DEFAULT '{}',
    technologies TEXT[] NOT NULL DEFAULT '{}',
    countries TEXT[] NOT NULL DEFAULT '{}',
    locations TEXT[] NOT NULL DEFAULT '{}',
    excluded_keywords TEXT[] NOT NULL DEFAULT '{}',
    remote TEXT NOT NULL DEFAULT 'any' CHECK (remote IN ('any', 'remote_only', 'remote_or_hybrid', 'onsite_ok')),
    seniority TEXT[] NOT NULL DEFAULT '{}',
    schedule_interval INTERVAL NOT NULL DEFAULT '12 hours' CHECK (schedule_interval >= INTERVAL '1 hour'),
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, name)
);
CREATE TRIGGER research_profiles_set_updated_at BEFORE UPDATE ON research_profiles
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();

-- Every workspace starts with a Full Stack Developer profile.
CREATE FUNCTION rolescout_default_research_profile() RETURNS trigger AS $$
BEGIN
    INSERT INTO research_profiles (workspace_id, name, title_keywords, technologies)
    VALUES (NEW.id, 'Full Stack Developer', ARRAY['full stack', 'fullstack', 'full-stack'],
            ARRAY['react', 'typescript', 'node.js', 'javascript', 'postgresql']);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER workspaces_default_research_profile AFTER INSERT ON workspaces
    FOR EACH ROW EXECUTE FUNCTION rolescout_default_research_profile();
INSERT INTO research_profiles (workspace_id, name, title_keywords, technologies)
SELECT id, 'Full Stack Developer', ARRAY['full stack', 'fullstack', 'full-stack'],
       ARRAY['react', 'typescript', 'node.js', 'javascript', 'postgresql']
FROM workspaces;

CREATE TABLE research_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    stage TEXT NOT NULL DEFAULT 'discovery'
        CHECK (stage IN ('discovery', 'detection', 'collection', 'scoring', 'result')),
    stages JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(stages) = 'object'),
    summary JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(summary) = 'object'),
    trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'schedule')),
    error TEXT,
    cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (workspace_id, profile_id) REFERENCES research_profiles (workspace_id, id) ON DELETE CASCADE,
    UNIQUE (workspace_id, id),
    CHECK ((state IN ('queued', 'running')) = (finished_at IS NULL))
);
CREATE INDEX research_runs_profile_idx ON research_runs (profile_id, queued_at DESC);
CREATE INDEX research_runs_queued_idx ON research_runs (queued_at) WHERE state = 'queued';
-- At most one queued or running run per profile.
CREATE UNIQUE INDEX research_runs_one_active_idx ON research_runs (profile_id) WHERE state IN ('queued', 'running');

CREATE TABLE research_run_events (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES research_runs (id) ON DELETE CASCADE,
    at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error')),
    stage TEXT NOT NULL,
    company TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL
);
CREATE INDEX research_run_events_run_idx ON research_run_events (run_id, id);

CREATE TABLE research_matches (
    profile_id UUID NOT NULL REFERENCES research_profiles (id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    score SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
    reasons JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(reasons) = 'array'),
    is_new BOOLEAN NOT NULL DEFAULT FALSE,
    first_matched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (profile_id, job_id)
);
CREATE INDEX research_matches_score_idx ON research_matches (profile_id, score DESC, job_id DESC);
CREATE INDEX research_matches_job_idx ON research_matches (job_id);

-- Progress markers of discovery sources, such as the last Hacker News thread read.
CREATE TABLE discovery_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
