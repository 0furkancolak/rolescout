-- Full-text search over jobs: title first, then company and technologies,
-- then the description. The "simple" configuration works for English and
-- Turkish alike. A trigger keeps the vector current, because a generated
-- column cannot use array_to_string (not immutable).

ALTER TABLE jobs ADD COLUMN search TSVECTOR NOT NULL DEFAULT ''::tsvector;

CREATE FUNCTION rolescout_jobs_search_vector(title TEXT, company TEXT, technologies TEXT[], description TEXT)
RETURNS tsvector AS $$
    SELECT setweight(to_tsvector('simple', coalesce(title, '')), 'A')
        || setweight(to_tsvector('simple', coalesce(company, '') || ' ' || array_to_string(technologies, ' ')), 'B')
        || setweight(to_tsvector('simple', left(coalesce(description, ''), 20000)), 'C')
$$ LANGUAGE sql STABLE;

CREATE FUNCTION rolescout_jobs_search() RETURNS trigger AS $$
BEGIN
    NEW.search := rolescout_jobs_search_vector(NEW.title, NEW.company_name, NEW.technologies, NEW.description);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER jobs_search BEFORE INSERT OR UPDATE OF title, company_name, technologies, description ON jobs
    FOR EACH ROW EXECUTE FUNCTION rolescout_jobs_search();

UPDATE jobs SET search = rolescout_jobs_search_vector(title, company_name, technologies, description);

CREATE INDEX jobs_search_idx ON jobs USING GIN (search);
CREATE INDEX jobs_country_idx ON jobs (country_code) WHERE closed_at IS NULL;
