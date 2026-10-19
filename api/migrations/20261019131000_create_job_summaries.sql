-- Short summaries the local model wrote of a job posting, one per language.
-- description_hash ties a summary to the text it was written from.
CREATE TABLE job_summaries (
    job_id UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    language TEXT NOT NULL CHECK (language IN ('tr', 'en')),
    description_hash TEXT NOT NULL,
    prompt_version INTEGER NOT NULL,
    model TEXT NOT NULL,
    summary TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_id, language)
);
