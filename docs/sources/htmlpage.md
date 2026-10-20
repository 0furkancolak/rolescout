# Source Review - job cards in plain HTML (`htmlpage`)

Reviewed: 2026-10-20
Adapter: `htmlpage` (`api/internal/ingest/htmlpage`)

## Access method
Some companies list their openings on their own career page with no job board
system and no structured data: a page of cards, each with a heading, a few
words and an "apply" button or link (for example `https://e-adam.com/kariyer`).
The adapter reads that one page, the same HTML any visitor receives, and turns
each card into a listing:

- a card is the largest block with exactly one apply button ("Hemen Başvur",
  "Başvur", "Apply", "Apply now") and a heading; the heading is the title;
- short lines such as "Tam Zamanlı", "Full time", "Ofis", "Remote", "Hibrit"
  give the employment type and the work arrangement; the first longer
  paragraph is the description; another short line is the location;
- the listing URL is the heading's link, else another link of the card, else
  the list page itself with a fragment from the title; the external ID is a
  hash of the title, so renaming a job makes a new listing;
- a page with no apply buttons is read as a plain list of links when its path
  names careers/jobs and at least two links lead to `/ilan/…`, `/job/…`,
  `/kariyer/…` pages.

Configuration: `{"url": "https://acme.example/kariyer", "company": "Acme"}`

The detector chooses this adapter only when no job board, embed or JSON-LD is
found and the page is a job list: its address talks about careers, or a card
names an employment type or a place of work. A marketing page that merely has
"apply" buttons (a loan page) stays "unknown".

## Terms and limits
- One request per sync, through the polite crawler: `robots.txt` is checked and
  `Crawl-delay` is honoured; the page is a public career page the company
  publishes for applicants.
- Rules only, no AI; layout changes can stop the reading. A page that yields no
  cards is reported as an incomplete snapshot, so no job is closed because of it.
- Pages that need a browser to render their list (client-side apps with an
  empty HTML shell) are not readable by this adapter.
- Descriptions are only the snippet on the card.

## Verification
Unit tests read a trimmed copy of the e-adam career page (eight cards), a
marketing page with apply buttons, a plain link list under `/kariyer`, and cards
with their own pages. Detector tests cover the choice between `htmlpage` and
"unknown".
