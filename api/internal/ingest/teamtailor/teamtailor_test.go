package teamtailor

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
	if err := a.Validate(map[string]any{"subdomain": "career"}); err != nil {
		t.Fatal(err)
	}
	if got := New(nil).base("career"); got != "https://career.teamtailor.com" {
		t.Errorf("base = %s", got)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/jobs.rss")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "career"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/jobs.rss" || len(got) != 2 {
		t.Fatalf("path %s, %d listings", path, len(got))
	}
	l := got[0]
	if l.ExternalID != "4ff07fd5-3ac0-4333-8f4c-955380396321" || l.Title != "UK Account Executive - SMB" ||
		l.URL != "https://career.teamtailor.com/jobs/8118064-uk-account-executive-smb" || l.Company != "Teamtailor" ||
		l.Location != "London, United Kingdom" || l.Arrangement != "hybrid" ||
		l.PostedAt == nil || l.PostedAt.Format("2006-01-02") != "2026-07-24" || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
	if got[1].Arrangement != "onsite" {
		t.Errorf("arrangement = %q", got[1].Arrangement)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "acme"})
		return err
	})
}
