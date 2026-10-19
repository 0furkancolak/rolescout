package lever

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"site": ""}, {"site": "a/b"}, {"site": "acme", "region": "us"}, {"site": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	for _, good := range []map[string]any{{"site": "acme"}, {"site": "acme", "region": "eu", "company": "Acme"}} {
		if err := a.Validate(good); err != nil {
			t.Errorf("config %v: %v", good, err)
		}
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/postings.json")(w, r)
	})
	a := &Adapter{Client: c, BaseURL: base}
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"site": "leverdemo", "company": "Lever Demo"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/v0/postings/leverdemo?mode=json" {
		t.Errorf("path = %s", path)
	}
	if len(got) != 2 {
		t.Fatalf("listings = %d", len(got))
	}
	l := got[0]
	if l.ExternalID != "ad208490-4052-4f91-a57e-433f2d1e484b" || l.Title != "Customer Success Manager" ||
		l.URL != "https://jobs.lever.co/leverdemo/ad208490-4052-4f91-a57e-433f2d1e484b" || l.Company != "Lever Demo" ||
		l.Location != "Atlanta, Georgia" || l.CountryCode != "US" || l.Arrangement != "hybrid" ||
		l.EmploymentType != "Regular Full Time (Salary)" || l.PostedAt == nil || l.PostedAt.Year() < 2020 || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
	if got[1].Arrangement != "onsite" {
		t.Errorf("second arrangement = %q", got[1].Arrangement)
	}
}

func TestFetchUsesCompanySiteWhenNameMissing(t *testing.T) {
	c, base := ingesttest.Server(t, ingesttest.File(t, "testdata/postings.json"))
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"site": "leverdemo"})
	if err != nil || got[0].Company != "leverdemo" {
		t.Fatalf("company = %q, err = %v", got[0].Company, err)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"site": "acme"})
		return err
	})
}

func TestLocationJoinsAllSites(t *testing.T) {
	if got := location("Istanbul", []string{"Istanbul", "Berlin", " "}); got != "Istanbul; Berlin" {
		t.Errorf("location = %q", got)
	}
	if got := location("", nil); got != "" {
		t.Errorf("empty location = %q", got)
	}
}
