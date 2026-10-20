# Company discovery sources

Discovery finds companies (their websites or career pages); detection then
identifies the job board behind each, and the matching adapter reads it. These
sources only name companies; they never read job listings. All requests go
through the polite crawler (robots.txt, pacing, identified user agent).

| Source | Origin | What it offers | Checked 2026-10-20 |
| --- | --- | --- | --- |
| Embedded seeds (`seeds/*.txt`) | `seed` | Career pages and company sites: `companies.txt` (boards), `turkey.txt` (boards hiring in Türkiye), `turkey_employers.txt` (about 250 Turkish employers: staffing agencies, banks, software houses, defence, telecom, retail, corporates) and `agencies_and_employers.txt` (about 170 staffing agencies, executive search firms, IT consultancies, software houses and large employers worldwide). Every address answered over HTTPS when the list was written. | yes |
| Probe by name | `probe` | Every company name of `probe_names.txt` plus the name of each seed that is not a career site is looked up on the job board systems' public APIs. | yes |
| DBpedia | `dbpedia` | Per country: companies and employing organisations (banks, airlines, publishers, broadcasters, universities, hospitals) whose website is on the country's domain, plus the country's "Companies of …" category two levels deep, its software, video game and IT consulting companies; clubs, events, people and works are left out. Then one worldwide pass over recruitment, staffing and consulting categories. About 980 for Türkiye. | yes |
| Y Combinator | `ycombinator` | The companies of the yc-oss public JSON that say they are hiring (about 1,490). | yes |
| Markdown lists | `list` | Community lists of companies: hiring-without-whiteboards, european-remote-software-companies, companies-hiring-security-remote, we-hire-remote-junior-devs. One list that moves does not stop the others. | yes |
| Hacker News, exchanges, sectors | `hackernews`, `probe` | See the source files. | |

## One site, one company
A company's bare home page is stored once however a source spells it
(`https://www.acme.example/`, `http://acme.example`): `discovery.SiteVariants`
lists the spellings and the company upsert matches any of them.

## Looked at and not used
- **Wikidata**: its robots.txt does not allow the query service for this client.
- **OpenStreetMap / Overpass**: too slow and heavy for company lists.
- **OpenCorporates, Crunchbase**: need a key or forbid automated use.
- **Job sites without an API** (eleman.net, yenibiris, secretcv, Indeed,
  LinkedIn): their terms or robots.txt do not allow it; see `kariyernet.md` for
  the one Turkish site that does.
- **GitHub lists with names only** (companies-hiring-security-remote at
  jaegeral, software-houses lists without links): no website to follow.
