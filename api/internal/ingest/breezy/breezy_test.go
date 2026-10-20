package breezy

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"subdomain": "a.b"}, {"subdomain": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"subdomain": "breezy"}); err != nil {
		t.Fatal(err)
	}
	if got := New(nil).base("breezy"); got != "https://breezy.breezy.hr" {
		t.Errorf("base = %s", got)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/json.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "breezy"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/json" || len(got) != 3 {
		t.Fatalf("path %s, %d listings", path, len(got))
	}
	l := got[0]
	if l.ExternalID != "98323abf2296" || l.Title != "Employee #12" || l.Company != "MS Breezy Trial" ||
		l.URL != "https://breezy.breezy.hr/p/98323abf2296-employee-12" || l.Location != "Chaos, FL" ||
		l.CountryCode != "US" || l.EmploymentType != "other" ||
		l.PostedAt == nil || l.PostedAt.Format("2006-01-02") != "2024-02-15" || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "acme"})
		return err
	})
}
