package weworkremotely

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

const fixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>We Work Remotely</title>
<item>
  <title>Stripe: Abuse Research Engineer</title>
  <region>Anywhere in the World</region>
  <country></country>
  <category>Full-Stack Programming</category>
  <type>Full-Time</type>
  <description>&lt;p&gt;Work with Go and Postgres&lt;/p&gt;</description>
  <pubDate>Tue, 20 Oct 2026 18:10:55 +0000</pubDate>
  <guid>https://weworkremotely.com/remote-jobs/stripe-abuse-research-engineer</guid>
  <link>https://weworkremotely.com/remote-jobs/stripe-abuse-research-engineer</link>
</item></channel></rss>`

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"category": "../x"}) == nil {
		t.Error("bad category accepted")
	}
	if err := a.Validate(map[string]any{"category": "remote-programming-jobs"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchSplitsCompanyFromTitle(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(fixture))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"category": "remote-programming-jobs"})
	if path != "/categories/remote-programming-jobs.rss" {
		t.Errorf("path = %s", path)
	}
	if err == nil || len(got) != 1 {
		t.Fatalf("got %d listings, err %v", len(got), err)
	}
	l := got[0]
	if l.Company != "Stripe" || l.Title != "Abuse Research Engineer" || l.Location != "Anywhere in the World" ||
		l.EmploymentType != "Full-Time" || l.Arrangement != "remote" || l.PostedAt == nil ||
		l.URL != "https://weworkremotely.com/remote-jobs/stripe-abuse-research-engineer" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}
