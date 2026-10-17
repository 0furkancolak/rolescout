package personio

import (
	"net/http"
	"strings"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"subdomain": "acme", "tld": "org"}, {"subdomain": "a/b"}, {"subdomain": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	for _, good := range []map[string]any{{"subdomain": "acme"}, {"subdomain": "acme", "tld": "com"}} {
		if err := a.Validate(good); err != nil {
			t.Errorf("config %v: %v", good, err)
		}
	}
	if got := New(nil).base("acme", ""); got != "https://acme.jobs.personio.de" {
		t.Errorf("base = %s", got)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/feed.xml")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "personio"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/xml?language=en" {
		t.Errorf("path = %s", path)
	}
	l := got[0]
	if l.ExternalID != "1834171" || l.Title != "Staff Software Engineer, Data Platform" ||
		l.URL != base+"/job/1834171" || l.Company != "Personio SE & Co. KG" || l.Location != "Munich, Berlin" ||
		l.EmploymentType != "full-time" || l.PostedAt == nil || !strings.Contains(l.Description, "Kafka") {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "acme"})
		return err
	})
}
