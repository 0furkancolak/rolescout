-- Career pages that print their job list in plain HTML ("htmlpage") and the
-- kariyer.net category lists ("kariyernet"). Both honour robots.txt through the
-- shared crawler; the reviews are written up in docs/sources/.
INSERT INTO source_adapter_reviews (adapter, enabled, reviewed_at, note) VALUES
    ('htmlpage', TRUE, NOW(), 'Job cards printed in the HTML of a company''s own career page; one page per source, robots.txt respected, no credentials; see docs/sources/htmlpage.md'),
    ('kariyernet', TRUE, NOW(), 'Public category lists of kariyer.net (cards only); /is-ilanlari/* is allowed by its robots.txt, paced and capped; see docs/sources/kariyernet.md');

-- One source per category list. They belong to no company: each card names its employer.
INSERT INTO job_sources (key, name, base_url, enabled, adapter, config, sync_interval) VALUES
    ('kariyernet-yazilim', 'kariyer.net: Yazılım', 'https://www.kariyer.net/is-ilanlari/yazilim', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/yazilim"}', INTERVAL '12 hours'),
    ('kariyernet-yazilim-gelistirici', 'kariyer.net: Yazılım Geliştirici', 'https://www.kariyer.net/is-ilanlari/yazilim-gelistirici', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/yazilim-gelistirici"}', INTERVAL '12 hours'),
    ('kariyernet-yazilim-muhendisi', 'kariyer.net: Yazılım Mühendisi', 'https://www.kariyer.net/is-ilanlari/yazilim-muhendisi', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/yazilim-muhendisi"}', INTERVAL '12 hours'),
    ('kariyernet-yazilim-uzmani', 'kariyer.net: Yazılım Uzmanı', 'https://www.kariyer.net/is-ilanlari/yazilim-uzmani', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/yazilim-uzmani"}', INTERVAL '12 hours'),
    ('kariyernet-mobil-yazilim-gelistirici', 'kariyer.net: Mobil Yazılım Geliştirici', 'https://www.kariyer.net/is-ilanlari/mobil-yazilim-gelistirici', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/mobil-yazilim-gelistirici"}', INTERVAL '12 hours'),
    ('kariyernet-bilgisayar-muhendisi', 'kariyer.net: Bilgisayar Mühendisi', 'https://www.kariyer.net/is-ilanlari/bilgisayar-muhendisi', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/bilgisayar-muhendisi"}', INTERVAL '12 hours'),
    ('kariyernet-bilgi-islem', 'kariyer.net: Bilgi İşlem', 'https://www.kariyer.net/is-ilanlari/bilgi-islem', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/bilgi-islem"}', INTERVAL '12 hours'),
    ('kariyernet-veri-bilimci', 'kariyer.net: Veri Bilimci', 'https://www.kariyer.net/is-ilanlari/veri-bilimci', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/veri-bilimci"}', INTERVAL '12 hours'),
    ('kariyernet-veri-analisti', 'kariyer.net: Veri Analisti', 'https://www.kariyer.net/is-ilanlari/veri-analisti', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/veri-analisti"}', INTERVAL '12 hours'),
    ('kariyernet-siber-guvenlik', 'kariyer.net: Siber Güvenlik', 'https://www.kariyer.net/is-ilanlari/siber-guvenlik', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/siber-guvenlik"}', INTERVAL '12 hours'),
    ('kariyernet-ui-ux-tasarimci', 'kariyer.net: UI/UX Tasarımcı', 'https://www.kariyer.net/is-ilanlari/ui-ux-tasarimci', TRUE, 'kariyernet', '{"url": "https://www.kariyer.net/is-ilanlari/ui-ux-tasarimci"}', INTERVAL '12 hours')
ON CONFLICT (key) DO NOTHING;
