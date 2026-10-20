-- Companies found in DBpedia (including recruitment agencies, software houses
-- and consultancies) and in the Y Combinator list of companies that are hiring.
-- "dbpedia" was written by the discovery source but missing from this list.
ALTER TABLE companies DROP CONSTRAINT companies_origin_check;
ALTER TABLE companies ADD CONSTRAINT companies_origin_check
    CHECK (origin IN ('seed', 'hackernews', 'list', 'link', 'manual', 'probe', 'dbpedia', 'ycombinator'));
