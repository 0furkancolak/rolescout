package discovery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/focus"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

// dbpediaServer answers each country with total rows, pageSize at a time.
func dbpediaServer(t *testing.T, total map[string]int) (*DBpedia, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("query")
		queries = append(queries, q)
		country := ""
		for c := range total {
			if strings.Contains(q, "# country "+c+"\n") {
				country = c
			}
		}
		var offset, limit int
		fmt.Sscanf(q[strings.Index(q, "LIMIT"):], "LIMIT %d OFFSET %d", &limit, &offset)
		var b strings.Builder
		b.WriteString(`{"results":{"bindings":[`)
		first := true
		for i := offset; i < min(offset+limit, total[country]); i++ {
			if !first {
				b.WriteString(",")
			}
			first = false
			site := fmt.Sprintf("https://www.%s-co%d.example/?utm=1", strings.ToLower(country), i)
			if i%7 == 0 {
				site = "https://www.facebook.com/page" // not a company's own site
			}
			fmt.Fprintf(&b, `{"site":{"value":%q},"name":{"value":"Co %s %d"}}`, site, country, i)
		}
		b.WriteString(`]}}`)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	c := crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "RoleScout/test", HostInterval: time.Millisecond, Transport: rewrite{target}})
	return &DBpedia{Client: c, PageSize: 10, MaxRows: 100, PagesPerRun: 2, Countries: []string{"TR"}}, &queries
}

func TestDBpediaReadsCountriesInPagesAndResumes(t *testing.T) {
	w, queries := dbpediaServer(t, map[string]int{"TR": 25, "DE": 5})
	ctx := focus.With(ingesttest.Ctx(t), []string{"DE"})

	var got []Candidate
	collect := func(c Candidate) error { got = append(got, c); return nil }
	state, err := w.Discover(ctx, "", collect)
	if err != nil {
		t.Fatal(err)
	}
	// Focus country first: DE is one short page; then TR gets one more page of the two allowed.
	if len(*queries) != 2 || !strings.Contains((*queries)[0], "# country DE") || !w.PendingCtx(ctx, state) {
		t.Fatalf("queries = %d, state = %s", len(*queries), state)
	}
	for _, c := range got {
		if strings.Contains(c.URL, "facebook") || strings.Contains(c.URL, "?") || c.Origin != "dbpedia" || c.Country == "" {
			t.Fatalf("candidate = %+v", c)
		}
	}
	for w.PendingCtx(ctx, state) {
		if state, err = w.Discover(ctx, state, collect); err != nil {
			t.Fatal(err)
		}
	}
	// 5 DE + 25 TR rows, minus every seventh (facebook) row of each country.
	want := (5 - 1) + (25 - 4)
	if len(got) != want {
		t.Fatalf("candidates = %d, want %d", len(got), want)
	}
	if w.PendingCtx(ingesttest.Ctx(t), state) {
		t.Fatal("without the focus country, only TR was needed and it is done")
	}
}

func TestDBpediaStartsOverWhenEverythingWasRead(t *testing.T) {
	w, queries := dbpediaServer(t, map[string]int{"TR": 3})
	ctx := ingesttest.Ctx(t)
	state, err := w.Discover(ctx, "", func(Candidate) error { return nil })
	if err != nil || w.PendingCtx(ctx, state) {
		t.Fatalf("state = %s, err = %v", state, err)
	}
	n := len(*queries)
	if _, err := w.Discover(ctx, state, func(Candidate) error { return nil }); err != nil || len(*queries) == n {
		t.Fatalf("a finished source is read again on the schedule: queries %d -> %d, err = %v", n, len(*queries), err)
	}
}

func TestCompanySite(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.acme.com.tr/tr/?a=1#x": "https://www.acme.com.tr/tr/",
		"http://acme.example":               "http://acme.example",
		"https://tr.linkedin.com/company/x": "",
		"https://www.facebook.com/acme":     "",
		"mailto:info@acme.example":          "",
		"https://localhost/":                "",
	} {
		if got := companySite(in); got != want {
			t.Errorf("companySite(%q) = %q, want %q", in, got, want)
		}
	}
}
