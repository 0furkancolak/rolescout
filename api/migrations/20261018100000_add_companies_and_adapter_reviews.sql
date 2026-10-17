-- Company catalog for career page discovery, adapter-type reviews and the
-- installation administrator (ADR 0004).

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET is_admin = TRUE
WHERE id = (SELECT id FROM users ORDER BY created_at, id LIMIT 1);

CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    domain TEXT,
    careers_url TEXT NOT NULL UNIQUE CHECK (careers_url ~ '^https?://'),
    ats TEXT NOT NULL DEFAULT 'unknown' CHECK (ats ~ '^[a-z0-9_]+$'),
    ats_id TEXT CHECK (ats_id IS NULL OR btrim(ats_id) <> ''),
    origin TEXT NOT NULL CHECK (origin IN ('commoncrawl', 'seed', 'link', 'manual')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'empty', 'unreachable', 'blocked_by_robots', 'disabled')),
    last_checked_at TIMESTAMPTZ,
    last_error TEXT,
    failure_count INTEGER NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (ats, ats_id)
);
CREATE INDEX companies_status_next_check_idx ON companies (status, next_check_at);
CREATE INDEX companies_name_idx ON companies (lower(name), id);
CREATE TRIGGER companies_set_updated_at BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();

ALTER TABLE job_sources
    ADD COLUMN company_id UUID UNIQUE REFERENCES companies (id) ON DELETE CASCADE;
ALTER TABLE jobs
    ADD COLUMN company_id UUID REFERENCES companies (id) ON DELETE SET NULL;
CREATE INDEX jobs_company_idx ON jobs (company_id);

CREATE TABLE workspace_company_blocks (
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    company_id UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, company_id)
);
CREATE INDEX workspace_company_blocks_company_idx ON workspace_company_blocks (company_id);

-- Reviews move from each source to the adapter type: thousands of discovered
-- companies use a handful of public job board APIs with the same terms.
CREATE TABLE source_adapter_reviews (
    adapter TEXT PRIMARY KEY CHECK (adapter ~ '^[a-z0-9_]+$'),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    reviewed_at TIMESTAMPTZ,
    note TEXT,
    CHECK (NOT enabled OR (reviewed_at IS NOT NULL AND btrim(coalesce(note, '')) <> ''))
);

INSERT INTO source_adapter_reviews (adapter, enabled, reviewed_at, note) VALUES
    ('greenhouse', TRUE, NOW(), 'Public Job Board API intended for publishing a company''s jobs; no credentials; terms checked 2026-10-18; see docs/sources/greenhouse.md'),
    ('lever', TRUE, NOW(), 'Public Postings API intended for publishing a company''s jobs; no credentials; terms checked 2026-10-18; see docs/sources/lever.md'),
    ('ashby', TRUE, NOW(), 'Public Job Postings API intended for publishing a company''s jobs; no credentials; terms checked 2026-10-18; see docs/sources/ashby.md'),
    ('workable', TRUE, NOW(), 'Public jobs widget API intended for embedding a company''s jobs; no credentials; terms checked 2026-10-18; see docs/sources/workable.md'),
    ('smartrecruiters', TRUE, NOW(), 'Public Posting API for a company''s published jobs; no credentials; terms checked 2026-10-18; see docs/sources/smartrecruiters.md'),
    ('recruitee', TRUE, NOW(), 'Public careers site API for a company''s published offers; no credentials; terms checked 2026-10-18; see docs/sources/recruitee.md'),
    ('personio', TRUE, NOW(), 'Public XML job feed of a company''s career page; no credentials; terms checked 2026-10-18; see docs/sources/personio.md'),
    ('bamboohr', TRUE, NOW(), 'Public careers list of a company''s career page; no credentials; terms checked 2026-10-18; see docs/sources/bamboohr.md'),
    ('jsonld', TRUE, NOW(), 'schema.org JobPosting data that sites publish for search engines; robots.txt respected; see docs/sources/jsonld.md'),
    ('workday', FALSE, NULL, NULL);

ALTER TABLE job_sources DROP CONSTRAINT job_sources_enabled_requires_review;

-- A source can be enabled when it has its own review or its adapter type has
-- an enabled review. Raised as the old constraint so callers keep mapping it.
CREATE FUNCTION rolescout_check_source_review() RETURNS trigger AS $$
BEGIN
    IF NEW.enabled
       AND NOT (NEW.reviewed_at IS NOT NULL AND btrim(coalesce(NEW.review_note, '')) <> '')
       AND NOT EXISTS (SELECT 1 FROM source_adapter_reviews r WHERE r.adapter = NEW.adapter AND r.enabled)
    THEN
        RAISE EXCEPTION 'source % must be reviewed before it is enabled', NEW.key
            USING ERRCODE = 'check_violation', CONSTRAINT = 'job_sources_enabled_requires_review';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER job_sources_check_review BEFORE INSERT OR UPDATE ON job_sources
    FOR EACH ROW EXECUTE FUNCTION rolescout_check_source_review();
