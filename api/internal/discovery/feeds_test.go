package discovery

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/focus"
	"example.com/rolescout/api/internal/ingest/ingesttest"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

const nasdaqSample = `Symbol|Security Name|Market Category|Test Issue|Financial Status|Round Lot Size|ETF|NextShares
AAPL|Apple Inc. - Common Stock|Q|N|N|100|N|N
ACME|Acme Holdings, Inc. - Common Stock|Q|N|N|100|N|N
ACMEW|Acme Holdings, Inc. - Warrant|Q|N|N|100|N|N
QQQ|Invesco QQQ Trust, Series 1|G|N|N|100|Y|N
ZTEST|Test Corp - Common Stock|Q|Y|N|100|N|N
SPAC|Bright Acquisition Corp - Units|Q|N|N|100|N|N
SAP|SAP SE - American Depositary Shares|Q|N|N|100|N|N
File Creation Time: 1020202600:00|||||||
`

func TestParseListingKeepsOperatingCompanies(t *testing.T) {
	got := strings.Join(ParseListing(nasdaqSample), ",")
	if got != "Apple,Acme" {
		t.Fatalf("names = %s", got)
	}
}

func TestCleanCompanyName(t *testing.T) {
	for in, want := range map[string]string{
		"Apple Inc. - Common Stock":       "Apple",
		"Acme Holdings, Inc.":             "Acme",
		"Globex Corporation Class A":      "Globex",
		"Initech Technologies Group Ltd.": "Initech",
		"Stripe":                          "Stripe",
	} {
		if got := cleanCompanyName(in); got != want {
			t.Errorf("cleanCompanyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExchangeFeedHandsOutBatchesAndFinishes(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(nasdaqSample)) })
	feed := &ExchangeFeed{Client: c, URLs: []string{base + "/a.txt"}, Batch: 1}
	names, next, more, err := feed.Next(context.Background(), "")
	if err != nil || len(names) != 1 || names[0] != "Apple" || next != "1" || !more {
		t.Fatalf("first = %v %q %v %v", names, next, more, err)
	}
	names, next, more, err = feed.Next(context.Background(), next)
	if err != nil || len(names) != 1 || names[0] != "Acme" || more {
		t.Fatalf("second = %v %q %v %v", names, next, more, err)
	}
}

func TestSectorFeedAsksOneSectorRegionPairPerStepAndSkipsWhenOff(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"companies":["Stripe","  Adyen ","stripe"]}`}, llmtest.Scripted{Content: `not json`})
	on := true
	feed := &SectorFeed{LLM: newSectorClient(t, srv), Enabled: func(context.Context) bool { return on }, PerRun: 2}
	names, next, more, err := feed.Next(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "Stripe,Adyen" || next != ":2" || !more {
		t.Fatalf("got %v %q %v", names, next, more)
	}
	// The questions are asked at the same time, so their order is not fixed.
	asked := ""
	for _, r := range srv.Requests() {
		asked += string(r.Body)
	}
	if len(srv.Requests()) != 2 || !strings.Contains(asked, "fintech and payments") || !strings.Contains(asked, "banking and insurance") {
		t.Fatalf("requests = %d: %s", len(srv.Requests()), asked)
	}
	on = false
	fs := &FeedSource{Feed: feed, Probe: &Probe{}}
	if fs.Pending(":2") {
		t.Error("a feed that is switched off must not keep discovery pending")
	}
	state, err := fs.Discover(context.Background(), ":2", func(Candidate) error { t.Error("no candidates while off"); return nil })
	if err != nil || state != ":2" {
		t.Errorf("off: state=%q err=%v", state, err)
	}
}

func newSectorClient(t *testing.T, srv *llmtest.Server) *llm.Client {
	t.Helper()
	return llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second})
}

func TestSectorFeedAsksTheProfilesCountriesFirstAndRestartsWhenTheyChange(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"companies":["Trendyol"]}`})
	feed := &SectorFeed{LLM: newSectorClient(t, srv), PerRun: 1}
	ctx := focus.With(context.Background(), []string{"TR"})
	names, next, more, err := feed.Next(ctx, "")
	if err != nil || len(names) != 1 || next != "TR:1" || !more {
		t.Fatalf("got %v %q %v %v", names, next, more, err)
	}
	if body := string(srv.Requests()[0].Body); !strings.Contains(body, "Türkiye") {
		t.Errorf("the first question should be about the focus country: %s", body)
	}
	// Progress recorded for another focus does not carry over.
	_, next, _, _ = feed.Next(focus.With(context.Background(), []string{"DE"}), "TR:7")
	if next != "DE:1" {
		t.Errorf("state after a focus change = %q", next)
	}
}

func TestCountryOfURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://careers.example.com.tr/jobs": "TR", "https://www.sap.de": "DE", "https://acme.com": "", "https://acme.co.uk/x": "GB", "": "",
	} {
		if got := CountryOfURL(in); got != want {
			t.Errorf("CountryOfURL(%q) = %q, want %q", in, got, want)
		}
	}
}
