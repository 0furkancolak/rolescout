package jsonld

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func page(scripts ...string) string {
	var b strings.Builder
	b.WriteString("<html><head><title>Careers</title>")
	for _, s := range scripts {
		b.WriteString(`<script type="application/ld+json">` + s + `</script>`)
	}
	b.WriteString("</head><body></body></html>")
	return b.String()
}

const posting = `{"@context":"https://schema.org","@type":"JobPosting","title":"Full Stack Developer",
"identifier":{"@type":"PropertyValue","name":"Acme","value":"FS-1"},
"url":"https://acme.example/careers/fs-1",
"datePosted":"2026-10-10","validThrough":"2099-01-01T00:00",
"employmentType":["FULL_TIME"],"jobLocationType":"TELECOMMUTE",
"hiringOrganization":{"@type":"Organization","name":"Acme"},
"jobLocation":{"@type":"Place","address":{"@type":"PostalAddress","addressLocality":"İstanbul","addressCountry":"TR"}},
"description":"&lt;p&gt;React &amp;amp; Node.js&lt;/p&gt;"}`

func TestExtractSingleObject(t *testing.T) {
	ps, _, err := ExtractJobPostings([]byte(page(posting)), "https://acme.example/careers")
	if err != nil || len(ps) != 1 {
		t.Fatalf("postings = %v, err = %v", ps, err)
	}
	p := ps[0]
	if p.ID != "FS-1" || p.Title != "Full Stack Developer" || p.URL != "https://acme.example/careers/fs-1" ||
		p.Company != "Acme" || p.Location != "İstanbul, TR" || p.CountryCode != "TR" || p.Arrangement != "remote" ||
		p.EmploymentType != "FULL_TIME" || p.PostedAt == nil || p.PostedAt.Format("2006-01-02") != "2026-10-10" {
		t.Errorf("posting = %+v", p)
	}
	if !strings.Contains(ingest.HTMLToText(p.Description), "React & Node.js") {
		t.Errorf("description = %q", p.Description)
	}
}

func TestExtractArrayGraphAndTypeArray(t *testing.T) {
	graph := `{"@context":"https://schema.org","@graph":[{"@type":"Organization","name":"Acme"},
		{"@type":["JobPosting"],"title":"Backend","url":"https://acme.example/jobs/2","hiringOrganization":"Acme"}]}`
	array := `[{"@type":"JobPosting","title":"Frontend","url":"https://acme.example/jobs/3","hiringOrganization":{"name":"Acme"}},
		{"@type":"WebSite","name":"x"}]`
	ps, _, err := ExtractJobPostings([]byte(page(graph, array)), "https://acme.example/careers")
	if err != nil || len(ps) != 2 {
		t.Fatalf("postings = %+v, err = %v", ps, err)
	}
	if ps[0].Title != "Backend" || ps[0].Company != "Acme" || ps[0].ID != "https://acme.example/jobs/2" || ps[1].Title != "Frontend" {
		t.Errorf("postings = %+v", ps)
	}
}

func TestExtractSkipsMalformedScriptAndExpired(t *testing.T) {
	expired := `{"@type":"JobPosting","title":"Old","url":"https://acme.example/jobs/old","validThrough":"2020-01-01"}`
	ps, _, err := ExtractJobPostings([]byte(page(`{"@type": "JobPosting", broken`, expired, posting)), "https://acme.example/careers")
	if err != nil || len(ps) != 1 || ps[0].Title != "Full Stack Developer" {
		t.Fatalf("postings = %+v, err = %v", ps, err)
	}
}

func TestExtractIDFallsBackToPageAndTitle(t *testing.T) {
	ps, _, _ := ExtractJobPostings([]byte(page(`{"@type":"JobPosting","title":"No URL"}`)), "https://acme.example/careers/x")
	if len(ps) != 1 || ps[0].URL != "https://acme.example/careers/x" || !strings.HasPrefix(ps[0].ID, "https://acme.example/careers/x#") {
		t.Fatalf("postings = %+v", ps)
	}
}

func TestExtractLinksUnderCareersPath(t *testing.T) {
	body := `<html><body>
		<a href="/careers/job-1">1</a><a href="job-2">2</a><a href="https://acme.example/careers/job-1#apply">dup</a>
		<a href="/blog/post">blog</a><a href="https://other.example/careers/x">other</a><a href="mailto:x@y">mail</a>
		<a href="/careers">self</a></body></html>`
	_, links, err := ExtractJobPostings([]byte(body), "https://acme.example/careers/")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(links, " "); got != "https://acme.example/careers/job-1 https://acme.example/careers/job-2" {
		t.Errorf("links = %s", got)
	}
}

// site serves a careers index linking to n job pages.
func site(t *testing.T, n int, extra map[string]string) (*Adapter, string, *atomic.Int32) {
	var hits atomic.Int32
	var base string
	c, base0 := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if body, ok := extra[r.URL.Path]; ok {
			if strings.HasPrefix(body, "PDF") {
				w.Header().Set("Content-Type", "application/pdf")
			}
			w.Write([]byte(body))
			return
		}
		switch {
		case r.URL.Path == "/careers":
			var b strings.Builder
			b.WriteString("<html><body>")
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&b, `<a href="/careers/job-%d">job</a>`, i)
			}
			b.WriteString(`<a href="/careers/brochure.pdf">pdf</a></body></html>`)
			w.Write([]byte(b.String()))
		case strings.HasPrefix(r.URL.Path, "/careers/job-"):
			id := strings.TrimPrefix(r.URL.Path, "/careers/job-")
			w.Write([]byte(page(fmt.Sprintf(`{"@type":"JobPosting","title":"Job %s","url":"%s/careers/job-%s","hiringOrganization":{"name":"Acme"}}`, id, base, id))))
		default:
			http.NotFound(w, r)
		}
	})
	base = base0
	return &Adapter{Client: c, MaxPages: 100}, base, &hits
}

func TestFetchCrawlsCareersPages(t *testing.T) {
	a, base, _ := site(t, 3, map[string]string{"/careers/brochure.pdf": "PDF data"})
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/careers"})
	// The PDF is skipped and noted; the snapshot is still complete.
	if err == nil || ingest.IsIncompleteSnapshot(err) {
		t.Fatalf("err = %v, want a note", err)
	}
	if len(got) != 3 || got[0].Company != "Acme" || got[0].URL == "" {
		t.Fatalf("listings = %+v", got)
	}
}

func TestFetchHonoursPageLimit(t *testing.T) {
	a, base, hits := site(t, 10, nil)
	a.MaxPages = 3
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/careers"})
	if !ingest.IsIncompleteSnapshot(err) {
		t.Fatalf("err = %v, want an incomplete snapshot", err)
	}
	if hits.Load() != 3 || len(got) != 2 {
		t.Errorf("pages fetched = %d, listings = %d; want 3 pages, 2 listings", hits.Load(), len(got))
	}
}

func TestFetchFailsWhenStartPageFails(t *testing.T) {
	a, base, _ := site(t, 0, nil)
	if _, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/missing"}); err == nil {
		t.Fatal("a missing career page must fail the run")
	}
}

func TestValidate(t *testing.T) {
	a := New(nil, 10)
	for _, bad := range []map[string]any{{}, {"url": "ftp://x"}, {"url": "/careers"}, {"url": "https://x", "y": "z"}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"url": "https://acme.example/careers", "company": "Acme"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchTemporaryFailureOfALinkedPageIsIncomplete(t *testing.T) {
	c, base2 := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/careers":
			w.Write([]byte(`<html><a href="/careers/ok">a</a><a href="/careers/busy">b</a><a href="/careers/gone">c</a></html>`))
		case "/careers/ok":
			w.Write([]byte(page(`{"@type":"JobPosting","title":"OK"}`)))
		case "/careers/busy":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	})
	got, err := (&Adapter{Client: c, MaxPages: 10}).Fetch(ingesttest.Ctx(t), map[string]any{"url": base2 + "/careers"})
	if len(got) != 1 || err == nil || !ingest.IsIncompleteSnapshot(err) || !strings.Contains(err.Error(), "1 page could not be read") {
		t.Fatalf("listings = %d err = %v", len(got), err)
	}
}

func TestFetchMissingLinkedPagesAreANoteNotIncomplete(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/careers":
			w.Write([]byte(`<html><a href="/careers/ok">a</a><a href="/careers/gone">b</a><a href="/careers/doc.pdf">c</a></html>`))
		case "/careers/ok":
			w.Write([]byte(page(`{"@type":"JobPosting","title":"OK"}`, `{"broken`)))
		case "/careers/doc.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
		default:
			http.NotFound(w, r)
		}
	})
	got, err := (&Adapter{Client: c, MaxPages: 10}).Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/careers"})
	if len(got) != 1 || err == nil || ingest.IsIncompleteSnapshot(err) {
		t.Fatalf("listings = %d err = %v", len(got), err)
	}
	for _, want := range []string{"1 page not found", "1 page not HTML", "1 malformed JSON-LD block"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("note %q misses %q", err, want)
		}
	}
}

func TestFetchPageLimitIsIncomplete(t *testing.T) {
	a, base, _ := site(t, 10, nil)
	a.MaxPages = 3
	_, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/careers"})
	if !ingest.IsIncompleteSnapshot(err) || !strings.Contains(err.Error(), "page limit") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchTimeBudgetKeepsWhatWasRead(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/careers":
			w.Write([]byte(page(`{"@type":"JobPosting","title":"First"}`) + `<a href="/careers/slow">s</a>`))
		default:
			time.Sleep(300 * time.Millisecond)
			w.Write([]byte(page(`{"@type":"JobPosting","title":"Slow"}`)))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	got, err := (&Adapter{Client: c, MaxPages: 10}).Fetch(ctx, map[string]any{"url": base + "/careers"})
	if len(got) != 1 || !ingest.IsIncompleteSnapshot(err) || !strings.Contains(err.Error(), "time budget") {
		t.Fatalf("listings = %d err = %v", len(got), err)
	}
}

func TestFetchNonHTMLStartPageFails(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	_, err := (&Adapter{Client: c, MaxPages: 10}).Fetch(ingesttest.Ctx(t), map[string]any{"url": base + "/careers"})
	if err == nil || ingest.IsIncompleteSnapshot(err) || !strings.Contains(err.Error(), "not an HTML page") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchTimeoutScalesWithPages(t *testing.T) {
	if got := New(nil, 200).FetchTimeout(nil); got < 10*time.Minute {
		t.Fatalf("timeout = %v", got)
	}
}
