-- The country a company hires in, so research can read the companies of the
-- countries a profile asks for first. Set from the career page's country domain
-- or the shipped lists when a company is discovered, and refreshed from the
-- countries of its jobs after each collection.
ALTER TABLE companies ADD COLUMN country CHAR(2) CHECK (country ~ '^[A-Z]{2}$');
CREATE INDEX companies_country_idx ON companies (country) WHERE country IS NOT NULL;

UPDATE companies c SET country = j.country
FROM (
    SELECT company_id, mode() WITHIN GROUP (ORDER BY country_code) AS country
    FROM jobs
    WHERE company_id IS NOT NULL AND country_code IS NOT NULL
    GROUP BY company_id
) j
WHERE c.id = j.company_id;
