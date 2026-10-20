-- Job board systems added in October 2026: Teamtailor (RSS), Breezy HR (JSON),
-- Rippling (board API) and Gem (job board API). Each review is written up in
-- docs/sources/; the crawler follows robots.txt for all of them.
INSERT INTO source_adapter_reviews (adapter, enabled, reviewed_at, note) VALUES
    ('teamtailor', TRUE, NOW(), 'Career site RSS feed (/jobs.rss) meant for aggregators; robots.txt allows it; no credentials; see docs/sources/teamtailor.md'),
    ('breezy', TRUE, NOW(), 'Career site JSON list (/json); robots.txt allows it; no credentials; see docs/sources/breezy.md'),
    ('rippling', TRUE, NOW(), 'Public ATS board list API used by ats.rippling.com; no robots.txt on the API host, no credentials; list only; see docs/sources/rippling.md'),
    ('gem', TRUE, NOW(), 'Gem job board API (Greenhouse-compatible, public); robots.txt unreadable (403) so no rules apply; no credentials; see docs/sources/gem.md')
ON CONFLICT (adapter) DO NOTHING;
