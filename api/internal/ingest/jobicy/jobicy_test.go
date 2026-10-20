package jobicy

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

const fixture = `{"apiVersion":"2.2.19","jobs":[{"id":156026,"url":"https://jobicy.com/jobs/156026-senior-software-engineer-agents-2","jobTitle":"Senior Software Engineer, Agents","companyName":"RevenueCat","jobIndustry":["Software Engineering"],"jobType":["Full-Time"],"jobGeo":"EMEA,  LATAM,  Canada","jobLevel":"Senior","jobExcerpt":"short","jobDescription":"<p>Go and Kotlin</p>","pubDate":"2026-10-20T15:27:08+00:00"}]}`

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"industry": "A B"}) == nil {
		t.Error("bad industry accepted")
	}
	if err := a.Validate(map[string]any{"industry": "engineering", "geo": "turkey"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var q string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.RawQuery
		w.Write([]byte(fixture))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"industry": "engineering"})
	if q != "count=100&industry=engineering" {
		t.Errorf("query = %s", q)
	}
	if err == nil || len(got) != 1 {
		t.Fatalf("got %d listings, err %v", len(got), err)
	}
	l := got[0]
	if l.ExternalID != "156026" || l.Company != "RevenueCat" || l.Location != "EMEA, LATAM, Canada" ||
		l.EmploymentType != "Full-Time" || l.Arrangement != "remote" || l.PostedAt == nil {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}
