-- Companies found by looking up their name on job board systems.
ALTER TABLE companies DROP CONSTRAINT companies_origin_check;
ALTER TABLE companies ADD CONSTRAINT companies_origin_check
    CHECK (origin IN ('seed', 'hackernews', 'list', 'link', 'manual', 'probe'));
