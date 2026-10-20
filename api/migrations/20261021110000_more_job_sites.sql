-- Job sites that list postings of many employers, read through their public APIs
-- or feeds (robots.txt respected, credit and links kept as their terms ask). The
-- reviews are written up in docs/sources/. Each feed is a window onto the newest
-- jobs, so the adapters never close jobs that left it.
INSERT INTO source_adapter_reviews (adapter, enabled, reviewed_at, note) VALUES
    ('arbeitnow', TRUE, NOW(), 'Arbeitnow free public job board API (/api/job-board-api); robots.txt allows it; link back to the listing; see docs/sources/arbeitnow.md'),
    ('himalayas', TRUE, NOW(), 'Himalayas public remote-jobs API (/jobs/api, cursor paging, 20 jobs per page); robots.txt allows it; links go to the Himalayas job page; see docs/sources/himalayas.md'),
    ('hnhiring', TRUE, NOW(), 'Job posts of the monthly Ask HN: Who is hiring? thread through the HN Search (Algolia) API; each post links to its HN comment; see docs/sources/hnhiring.md'),
    ('jobicy', TRUE, NOW(), 'Jobicy public remote-jobs API (/api/v2/remote-jobs); credit and direct link kept; robots.txt allows it; see docs/sources/jobicy.md'),
    ('remoteok', TRUE, NOW(), 'Remote OK public API (/api); every job links to its Remote OK page as the API terms require; see docs/sources/remoteok.md'),
    ('themuse', TRUE, NOW(), 'The Muse public jobs API (/api/public/jobs, no key); robots.txt allows it; see docs/sources/themuse.md'),
    ('weworkremotely', TRUE, NOW(), 'We Work Remotely public RSS feeds (all jobs and category feeds); robots.txt allows them; see docs/sources/weworkremotely.md'),
    ('workingnomads', TRUE, NOW(), 'Working Nomads public job feed (/api/exposed_jobs/); robots.txt allows it; see docs/sources/workingnomads.md');

-- These belong to no company: each listing names its own employer.
INSERT INTO job_sources (key, name, base_url, enabled, adapter, config, sync_interval) VALUES
    ('remoteok', 'Remote OK', 'https://remoteok.com', TRUE, 'remoteok', '{}', INTERVAL '6 hours'),
    ('arbeitnow', 'Arbeitnow', 'https://www.arbeitnow.com', TRUE, 'arbeitnow', '{"max_pages": "3"}', INTERVAL '6 hours'),
    ('himalayas', 'Himalayas', 'https://himalayas.app/jobs', TRUE, 'himalayas', '{"max_pages": "15"}', INTERVAL '6 hours'),
    ('workingnomads', 'Working Nomads', 'https://www.workingnomads.com/jobs', TRUE, 'workingnomads', '{}', INTERVAL '6 hours'),
    ('hnhiring', 'Hacker News: Who is hiring?', 'https://news.ycombinator.com/jobs', TRUE, 'hnhiring', '{"threads": "2"}', INTERVAL '12 hours'),
    ('weworkremotely-programming', 'We Work Remotely: Programming', 'https://weworkremotely.com/categories/remote-programming-jobs', TRUE, 'weworkremotely', '{"category": "remote-programming-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-full-stack', 'We Work Remotely: Full-Stack Programming', 'https://weworkremotely.com/categories/remote-full-stack-programming-jobs', TRUE, 'weworkremotely', '{"category": "remote-full-stack-programming-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-front-end', 'We Work Remotely: Front-End Programming', 'https://weworkremotely.com/categories/remote-front-end-programming-jobs', TRUE, 'weworkremotely', '{"category": "remote-front-end-programming-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-back-end', 'We Work Remotely: Back-End Programming', 'https://weworkremotely.com/categories/remote-back-end-programming-jobs', TRUE, 'weworkremotely', '{"category": "remote-back-end-programming-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-devops', 'We Work Remotely: DevOps and Sysadmin', 'https://weworkremotely.com/categories/remote-devops-sysadmin-jobs', TRUE, 'weworkremotely', '{"category": "remote-devops-sysadmin-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-design', 'We Work Remotely: Design', 'https://weworkremotely.com/categories/remote-design-jobs', TRUE, 'weworkremotely', '{"category": "remote-design-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-product', 'We Work Remotely: Product', 'https://weworkremotely.com/categories/remote-product-jobs', TRUE, 'weworkremotely', '{"category": "remote-product-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-management', 'We Work Remotely: Management and Finance', 'https://weworkremotely.com/categories/remote-management-and-finance-jobs', TRUE, 'weworkremotely', '{"category": "remote-management-and-finance-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-support', 'We Work Remotely: Customer Support', 'https://weworkremotely.com/categories/remote-customer-support-jobs', TRUE, 'weworkremotely', '{"category": "remote-customer-support-jobs"}', INTERVAL '8 hours'),
    ('weworkremotely-marketing', 'We Work Remotely: Sales and Marketing', 'https://weworkremotely.com/categories/remote-sales-and-marketing-jobs', TRUE, 'weworkremotely', '{"category": "remote-sales-and-marketing-jobs"}', INTERVAL '8 hours'),
    ('jobicy-all', 'Jobicy: All industries', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{}', INTERVAL '12 hours'),
    ('jobicy-engineering', 'Jobicy: Engineering', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "engineering"}', INTERVAL '12 hours'),
    ('jobicy-data-science', 'Jobicy: Data Science', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "data-science"}', INTERVAL '12 hours'),
    ('jobicy-design', 'Jobicy: Design and Multimedia', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "design-multimedia"}', INTERVAL '12 hours'),
    ('jobicy-business', 'Jobicy: Business', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "business"}', INTERVAL '12 hours'),
    ('jobicy-marketing', 'Jobicy: Marketing', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "marketing"}', INTERVAL '12 hours'),
    ('jobicy-support', 'Jobicy: Customer Support', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "supporting"}', INTERVAL '12 hours'),
    ('jobicy-admin', 'Jobicy: Admin Support', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "admin-support"}', INTERVAL '12 hours'),
    ('jobicy-copywriting', 'Jobicy: Copywriting', 'https://jobicy.com/remote-jobs', TRUE, 'jobicy', '{"industry": "copywriting"}', INTERVAL '12 hours'),
    ('themuse-software', 'The Muse: Software Engineering', 'https://www.themuse.com/search', TRUE, 'themuse', '{"category": "Software Engineering", "location": "Flexible / Remote", "max_pages": "10"}', INTERVAL '12 hours'),
    ('themuse-data', 'The Muse: Data and Analytics', 'https://www.themuse.com/search', TRUE, 'themuse', '{"category": "Data and Analytics", "location": "Flexible / Remote", "max_pages": "10"}', INTERVAL '12 hours'),
    ('themuse-it', 'The Muse: Computer and IT', 'https://www.themuse.com/search', TRUE, 'themuse', '{"category": "Computer and IT", "location": "Flexible / Remote", "max_pages": "10"}', INTERVAL '12 hours'),
    ('themuse-design', 'The Muse: Design and UX', 'https://www.themuse.com/search', TRUE, 'themuse', '{"category": "Design and UX", "location": "Flexible / Remote", "max_pages": "10"}', INTERVAL '12 hours'),
    ('themuse-product', 'The Muse: Product Management', 'https://www.themuse.com/search', TRUE, 'themuse', '{"category": "Product Management", "location": "Flexible / Remote", "max_pages": "10"}', INTERVAL '12 hours')
ON CONFLICT (key) DO NOTHING;
