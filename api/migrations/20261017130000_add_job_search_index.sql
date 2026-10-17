-- Job search pages by (last_seen_at, id) descending with keyset pagination.
CREATE INDEX jobs_last_seen_desc_idx ON jobs (last_seen_at DESC, id DESC);
DROP INDEX jobs_last_seen_idx;
