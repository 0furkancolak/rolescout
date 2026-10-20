# Job board systems that were reviewed and not added

Reviewed: 2026-10-20. Each was checked for a public, documented list of
published jobs that may be read by a crawler that follows `robots.txt`.

| System | Result |
| --- | --- |
| SmartRecruiters | The Posting API is public, but `robots.txt` of `api.smartrecruiters.com` disallows every crawler except LinkedIn's. Not added. |
| Pinpoint | The documented `/postings.json` is only served for tenants that enabled it; none of the sampled tenants answered with JSON, so the adapter could not be verified. Not added. |
| Comeet | The careers API needs a company token that is not public. Not added. |
| Join (join.com) | No documented public list API; the pages are rendered by a framework. Reading them is a job for `jsonld`/`htmlpage`. Not added. |
| Jobvite | Lists are HTML pages behind a redirecting app host, XML feeds need the customer's key. Not added. |
| Freshteam | The career page needs a browser (503/JS), the API needs the customer's key. Not added. |
| HiBob | `/api/job-ad` answers 401 without the customer's key. Not added. |
| Homerun | No public list API found; `api.homerun.co` needs a key. Not added. |
| iCIMS, SuccessFactors, Taleo | Customer-specific portals without a public, documented feed. Not added. |
| Zoho Recruit, Dayforce | Customer-specific pages or keyed APIs. Not added. |
