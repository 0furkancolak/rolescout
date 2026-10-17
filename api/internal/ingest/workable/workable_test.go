package workable

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"account": "../x"}, {"account": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"account": "hugging-face"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/account.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"account": "huggingface"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/v1/widget/accounts/huggingface?details=true" {
		t.Errorf("path = %s", path)
	}
	if len(got) != 2 {
		t.Fatalf("listings = %d", len(got))
	}
	l := got[0]
	if l.ExternalID != "81B46579FE" || l.Title != "Open-Source Machine Learning Engineer - EMEA Remote" ||
		l.URL != "https://apply.workable.com/j/81B46579FE" || l.Company != "Hugging Face" ||
		l.Location != "Paris, Île-de-France, France" || l.CountryCode != "FR" || l.Arrangement != "remote" ||
		l.EmploymentType != "Full-time" || l.PostedAt == nil || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"account": "acme"})
		return err
	})
}
