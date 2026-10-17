-- Jobs that disappear from their sources are closed instead of lingering, and
-- jobs carry a country when it is known (career page research).

ALTER TABLE job_source_listings ADD COLUMN closed_at TIMESTAMPTZ;
ALTER TABLE jobs
    ADD COLUMN closed_at TIMESTAMPTZ,
    ADD COLUMN country_code CHAR(2) CHECK (country_code ~ '^[A-Z]{2}$');

CREATE INDEX jobs_open_first_seen_idx ON jobs (first_seen_at DESC, id DESC) WHERE closed_at IS NULL;
CREATE INDEX jobs_closed_idx ON jobs (closed_at) WHERE closed_at IS NOT NULL;
CREATE INDEX job_source_listings_source_open_idx ON job_source_listings (source_id) WHERE closed_at IS NULL;
