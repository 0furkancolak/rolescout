-- The newest validated model result per job whose text still matches the job's
-- description, with the fields search and scoring can use. Rule-derived values
-- on jobs always win; these only fill gaps and add technologies (ADR 0005).
CREATE VIEW job_ai AS
SELECT DISTINCT ON (e.job_id)
    e.job_id,
    e.model,
    e.created_at,
    NULLIF(e.data ->> 'seniority', 'unknown') AS seniority,
    NULLIF(e.data ->> 'work_arrangement', 'unknown') AS work_arrangement,
    NULLIF(e.data ->> 'country_code', '') AS country_code,
    COALESCE(ARRAY(SELECT jsonb_array_elements_text(e.data -> 'technologies')), '{}'::text[]) AS technologies,
    NULLIF(e.data ->> 'summary', '') AS summary,
    NULLIF(e.data ->> 'remote_policy', '') AS remote_policy,
    e.data
FROM job_enrichments e
JOIN jobs j ON j.id = e.job_id AND j.description_hash = e.description_hash
WHERE e.status = 'ok' AND jsonb_typeof(e.data -> 'technologies') = 'array'
ORDER BY e.job_id, e.created_at DESC;
