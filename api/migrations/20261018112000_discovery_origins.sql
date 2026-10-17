-- Common Crawl's index and data hosts disallow crawlers in robots.txt, so
-- discovery uses Hacker News "Who is hiring?" threads and public company
-- lists instead (ADR 0004).
ALTER TABLE companies DROP CONSTRAINT companies_origin_check;
ALTER TABLE companies ADD CONSTRAINT companies_origin_check
    CHECK (origin IN ('seed', 'hackernews', 'list', 'link', 'manual'));
