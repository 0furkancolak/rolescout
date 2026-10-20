package rippling

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"board": "a/b"}, {"board": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"board": "rippling", "company": "Rippling"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/jobs.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "rippling", "company": "Rippling"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/platform/api/ats/v1/board/rippling/jobs" || len(got) != 3 {
		t.Fatalf("path %s, %d listings", path, len(got))
	}
	l := got[1]
	if l.ExternalID != "528f45e5-4c17-4fb7-affe-41e0c12c7c15" || l.Title != "Account Executive, Broker Channel (Dallas)" ||
		l.Company != "Rippling" || l.Location != "Remote (Dallas, Texas, US)" ||
		l.URL != "https://ats.rippling.com/rippling/jobs/528f45e5-4c17-4fb7-affe-41e0c12c7c15" || l.Description != "<p>Sales</p>" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "acme"})
		return err
	})
}
