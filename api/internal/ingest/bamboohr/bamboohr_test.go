package bamboohr

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
	if err := a.Validate(map[string]any{"subdomain": "gitkraken", "company": "GitKraken"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/list.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "gitkraken", "company": "GitKraken"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/careers/list" {
		t.Errorf("path = %s", path)
	}
	if len(got) != 3 {
		t.Fatalf("listings = %d", len(got))
	}
	l := got[0]
	if l.ExternalID != "178" || l.Title != "Applied Data Scientist" || l.URL != base+"/careers/178" ||
		l.Company != "GitKraken" || l.Location != "Scottsdale, Arizona, United States" || l.CountryCode != "United States" ||
		l.Arrangement != "remote" || l.EmploymentType != "Full-Time" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "acme"})
		return err
	})
}

func TestRedirectToMarketingSiteIsNotFound(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/careers/list" {
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}
		w.Write([]byte("<html>marketing</html>"))
	})
	_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"subdomain": "gone"})
	if err == nil {
		t.Fatal("a redirect to a web page must fail")
	}
}
