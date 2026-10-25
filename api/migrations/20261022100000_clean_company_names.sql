-- Companies found in "Who is hiring?" comments were named after the comment's
-- first line: "xAI (Yes, we just got acquired…)", "Hi friends, I run a small…".
-- Keep the part that is the name; a sentence with no name in it gets the job
-- board's id or the site's name. The same rules run when jobs are ingested
-- (ingest.CleanCompanyName), so these names do not come back.
CREATE TEMP TABLE renamed ON COMMIT DROP AS
WITH cleaned AS (
    SELECT id,
        btrim(regexp_replace(
            regexp_replace(
                regexp_replace(
                    regexp_replace(name, '\(?\s*(https?://|www\.)\S*\s*\)?', ' ', 'gi'),
                    '\s*\([^)]*\)?', ' ', 'g'),
                '\s+', ' ', 'g'),
            '(\s+[-–—]\s+|\s*[:|]\s+|\s*/\s+|,\s+|\s+(is|are|we|was|has|have|hiring|looking|building)\s+).*$', ''),
            ' -–—:|,.;/') AS short,
        name,
        ats_id,
        substring(regexp_replace(careers_url, '^https?://((www|jobs|careers|career|apply|boards|job-boards|hire|work|join)\.)*', '') FROM '^[^/:?#.]+') AS site
    FROM companies
    WHERE length(name) > 24 OR name ~ '[(|]' OR name ~* 'https?://|www\.'
), fixed AS (
    SELECT id, name,
        CASE
            WHEN short = '' OR length(short) > 60 OR short ~ ':' OR cardinality(string_to_array(short, ' ')) > 7
                OR short ~* '^(hi|hello|hey|we|we.re|i|i.m|how|looking|hiring|please|are you|do you|join|our|at|location|company|senior|staff|jobs?)\M'
                THEN initcap(replace(replace(CASE WHEN ats_id ~ '^[A-Za-z0-9_-]+$' THEN ats_id ELSE site END, '-', ' '), '_', ' '))
            ELSE short
        END AS new_name
    FROM cleaned
)
SELECT f.id, f.name AS old_name, f.new_name FROM fixed f
WHERE f.new_name <> '' AND f.new_name <> f.name AND length(f.new_name) < length(f.name);

UPDATE companies c SET name = r.new_name, updated_at = now() FROM renamed r WHERE c.id = r.id;

-- Their sources and jobs follow, so the lists show the clean name at once.
UPDATE job_sources s SET config = jsonb_set(s.config, '{company}', to_jsonb(r.new_name))
FROM renamed r
WHERE s.company_id = r.id AND s.config->>'company' = r.old_name;

UPDATE jobs j SET company_name = r.new_name FROM renamed r WHERE j.company_id = r.id AND j.company_name = r.old_name;
