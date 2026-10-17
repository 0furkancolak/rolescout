package recruitee

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
	if err := a.Validate(map[string]any{"subdomain": "channable"}); err != nil {
		t.Fatal(err)
	}
}

func TestBaseURLFromSubdomain(t *testing.T) {
	if got := New(nil).base("channable"); got != "https://channable.recruitee.com" {
		t.Errorf("base = %s", got)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/offers.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "channable"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/offers/" {
		t.Errorf("path = %s", path)
	}
	l := got[0]
	if l.ExternalID != "2786321" || l.Title != "Senior Customer Success Manager Benelux" ||
		l.URL != "https://jobs.channable.com/o/senior-customer-success-manager-benelux-9" || l.Company != "Channable" ||
		l.Location != "Utrecht, Utrecht, Netherlands" || l.CountryCode != "NL" || l.Arrangement != "hybrid" ||
		l.PostedAt == nil || l.PostedAt.Format("2006-01-02") != "2026-10-16" || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "acme"})
		return err
	})
}
