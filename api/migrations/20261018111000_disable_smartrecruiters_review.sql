-- SmartRecruiters' API host disallows all crawlers except LinkedIn in its
-- robots.txt, so RoleScout does not ship an adapter for it (ADR 0004).
UPDATE source_adapter_reviews
SET enabled = FALSE,
    reviewed_at = NOW(),
    note = 'Not supported: robots.txt of api.smartrecruiters.com disallows crawlers other than LinkedInBot (checked 2026-10-18)'
WHERE adapter = 'smartrecruiters';
