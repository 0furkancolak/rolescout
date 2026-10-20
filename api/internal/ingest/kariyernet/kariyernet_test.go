package kariyernet

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

type rewrite struct{ target *url.URL }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Host = req.URL.Host
	out.URL.Scheme, out.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(out)
}

func adapter(t *testing.T, h http.HandlerFunc) *Adapter {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	c := crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "RoleScout/test", HostInterval: time.Millisecond, Transport: rewrite{target}})
	return &Adapter{Client: c}
}

func TestParsePageReadsCards(t *testing.T) {
	body, err := os.ReadFile("testdata/list.html")
	if err != nil {
		t.Fatal(err)
	}
	cards, next := parsePage(body, "https://www.kariyer.net/is-ilanlari/yazilim", "/is-ilanlari/yazilim")
	if len(cards) != 6 || next != "https://www.kariyer.net/is-ilanlari/yazilim-2" {
		t.Fatalf("cards = %d next = %q", len(cards), next)
	}
	var found bool
	for _, c := range cards {
		if c.ExternalID == "4579428" {
			found = true
			if c.Title != "Çevre Mühendisi" || !strings.HasPrefix(c.Company, "Mitto") || c.Location != "Ankara" ||
				c.Arrangement != "onsite" || c.EmploymentType != "Tam zamanlı" || c.CountryCode != "TR" ||
				c.URL != "https://www.kariyer.net/is-ilani/mitto-consultancy-danismanlik-anonim-sirketi-cevre-muhendisi-4579428" {
				t.Fatalf("card = %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("card 4579428 missing")
	}
}

func card(id int, company string) string {
	return fmt.Sprintf(`<div data-test="ad-card" positionName="Dev %d" cityName="Ankara" workModelText="Hibrit" workTypeText="Tam zamanlı" countryName="Türkiye">
		<a href="/is-ilani/%s-dev-%d"><span data-test="ad-card-title">Dev %d</span><span data-test="subtitle">%s</span><span data-test="location">Ankara</span></a></div>`, id, company, id, id, company)
}

func list(next string, ids ...int) string {
	var b strings.Builder
	b.WriteString("<html><head>")
	if next != "" {
		b.WriteString(`<link rel="next" href="` + next + `">`)
	}
	b.WriteString("</head><body>")
	for _, id := range ids {
		b.WriteString(card(id, "acme"))
	}
	b.WriteString("</body></html>")
	return b.String()
}

func TestFetchFollowsNextPagesAndStops(t *testing.T) {
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.NotFound(w, r)
		case "/is-ilanlari/yazilim":
			fmt.Fprint(w, list("https://www.kariyer.net/is-ilanlari/yazilim-2", 1, 2))
		case "/is-ilanlari/yazilim-2":
			fmt.Fprint(w, list("https://www.kariyer.net/is-ilanlari?cp=3", 3))
		default:
			http.NotFound(w, r)
		}
	})
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yazilim"})
	if err != nil || len(got) != 3 {
		t.Fatalf("listings = %d, err = %v", len(got), err)
	}
	if got[2].ExternalID != "3" || got[2].Company != "acme" {
		t.Fatalf("listing = %+v", got[2])
	}
}

func TestFetchPageLimitAndFailuresAreIncomplete(t *testing.T) {
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/is-ilanlari/yazilim":
			fmt.Fprint(w, list("https://www.kariyer.net/is-ilanlari/yazilim-2", 1))
		case "/is-ilanlari/yazilim-2":
			http.Error(w, "slow down", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	})
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yazilim"})
	if len(got) != 1 || err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("listings = %d, err = %v", len(got), err)
	}

	b := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		n := 0
		fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/is-ilanlari/yazilim-"), "%d", &n)
		next := fmt.Sprintf("https://www.kariyer.net/is-ilanlari/yazilim-%d", n+1)
		if r.URL.Path == "/is-ilanlari/yazilim" {
			n, next = 1, "https://www.kariyer.net/is-ilanlari/yazilim-2"
		}
		fmt.Fprint(w, list(next, n*10, n*10+1))
	})
	got, err = b.Fetch(ingesttest.Ctx(t), map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yazilim", "max_pages": "2"})
	if len(got) != 4 || err == nil || !strings.Contains(err.Error(), "page limit") {
		t.Fatalf("listings = %d, err = %v", len(got), err)
	}
}

func TestFirstPageErrors(t *testing.T) {
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/is-ilanlari/yok" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "limit", http.StatusForbidden)
	})
	if _, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yok"}); err == nil {
		t.Fatal("a missing list must fail")
	}
	_, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yazilim"})
	if !ingest.IsRetryable(err) {
		t.Fatalf("a 403 should be retried later: %v", err)
	}
}

func TestValidate(t *testing.T) {
	a := &Adapter{}
	for _, bad := range []map[string]any{
		{"url": "https://evil.example/is-ilanlari/yazilim"},
		{"url": "https://www.kariyer.net/filtre"},
		{"url": "https://www.kariyer.net/is-ilanlari/yazilim", "max_pages": "x"},
		{},
	} {
		if a.Validate(bad) == nil {
			t.Errorf("%v must be rejected", bad)
		}
	}
	if err := a.Validate(map[string]any{"url": "https://www.kariyer.net/is-ilanlari/yazilim", "max_pages": "5"}); err != nil {
		t.Fatal(err)
	}
}
