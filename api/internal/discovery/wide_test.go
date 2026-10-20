package discovery

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestYCombinatorOffersHiringCompanies(t *testing.T) {
	body := `[
	  {"name":"Acme AI","website":"https://www.acme.ai/?ref=yc","all_locations":"San Francisco, CA, USA","status":"Active"},
	  {"name":"Bosphorus","website":"https://bosphorus.example","all_locations":"Istanbul, Türkiye; London, UK","status":"Active"},
	  {"name":"Gone","website":"https://gone.example","status":"Inactive"},
	  {"name":"Social","website":"https://www.linkedin.com/company/social","status":"Active"},
	  {"name":"No site","website":"","status":"Active"}
	]`
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
	got, _ := collect(t, &YCombinator{Client: c, URL: base + "/hiring.json"}, "")
	if len(got) != 2 {
		t.Fatalf("candidates = %+v", got)
	}
	acme, bos := byURL(got, "https://www.acme.ai/"), byURL(got, "https://bosphorus.example")
	if acme.Name != "Acme AI" || acme.Origin != "ycombinator" || acme.Country != "US" {
		t.Errorf("acme = %+v", acme)
	}
	if bos.Country != "TR" {
		t.Errorf("bosphorus country = %q, want TR", bos.Country)
	}
}

func TestMarkdownListSurvivesADeadListAndDropsProfiles(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone.md" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("- [Acme](https://acme.example/careers?utm=1)\n- [Acme on LinkedIn](https://www.linkedin.com/company/acme)\n"))
	})
	var got []Candidate
	_, err := (&MarkdownList{Client: c, URLs: []string{base + "/gone.md", base + "/ok.md"}}).Discover(ingesttest.Ctx(t), "", func(c Candidate) error {
		got = append(got, c)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "gone.md") {
		t.Errorf("the dead list must be reported, got %v", err)
	}
	if u := urls(got); u != "https://acme.example/careers" {
		t.Errorf("candidates = %s", u)
	}
}

func TestDBpediaAlsoAsksForOrganisationsCategoriesAndAgencies(t *testing.T) {
	w, queries := dbpediaServer(t, map[string]int{"TR": 3, "agencies": 4})
	w.PagesPerRun = 10
	got, state := collect(t, w, "")
	if state == "" || w.PendingCtx(ingesttest.Ctx(t), state) {
		t.Fatalf("everything should be read, state %q", state)
	}
	joined := strings.Join(*queries, "\n---\n")
	for _, want := range []string{"dbo:Organisation", "dbc:Companies_of_Turkey", "dbc:Employment_agencies", "dbc:Staffing_companies"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no query mentions %s", want)
		}
	}
	// The agencies are not tied to one country.
	for _, c := range got {
		if strings.Contains(c.Name, "agencies") && c.Country != "" {
			t.Errorf("agency %+v got a country", c)
		}
	}
}

func TestEmbeddedSeedListsAreWideAndDistinct(t *testing.T) {
	seeds, err := Seeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) < 500 {
		t.Errorf("only %d seeds", len(seeds))
	}
	var turkish int
	for _, s := range seeds {
		if s.Country == "TR" {
			turkish++
		}
	}
	if turkish < 200 {
		t.Errorf("only %d Turkish seeds", turkish)
	}
	names := ProbeNames()
	for _, want := range []string{"Adecco", "Randstad", "Michael Page", "Hays"} {
		if !slices.Contains(names, want) {
			t.Errorf("agency %q is not probed by name", want)
		}
	}
	for _, n := range names {
		if strings.HasSuffix(n, "Kariyer") || strings.HasSuffix(n, "Türkiye") {
			t.Errorf("probe name %q is a label, not a company", n)
		}
	}
}

func TestSiteVariantsCoverTheSpellingsOfAHomePage(t *testing.T) {
	got := SiteVariants("https://www.acme.example/")
	for _, want := range []string{"https://acme.example", "https://acme.example/", "http://www.acme.example", "http://acme.example/", "https://www.acme.example"} {
		if !slices.Contains(got, want) {
			t.Errorf("%s missing from %v", want, got)
		}
	}
	if slices.Contains(got, "https://www.acme.example/") {
		t.Error("the URL itself is not its own variant")
	}
	for _, notHome := range []string{"https://acme.example/careers", "https://acme.example/?x=1", "ftp://acme.example", "not a url"} {
		if v := SiteVariants(notHome); v != nil {
			t.Errorf("SiteVariants(%q) = %v, want nil", notHome, v)
		}
	}
}
