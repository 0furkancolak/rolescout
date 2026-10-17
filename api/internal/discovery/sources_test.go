package discovery

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func collect(t *testing.T, s Source, state string) ([]Candidate, string) {
	t.Helper()
	var got []Candidate
	next, err := s.Discover(ingesttest.Ctx(t), state, func(c Candidate) error {
		got = append(got, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, next
}

func urls(cs []Candidate) string {
	var out []string
	for _, c := range cs {
		out = append(out, c.URL)
	}
	return strings.Join(out, " ")
}

// hnServer serves the whoishiring user, three stories and their comments.
func hnServer(t *testing.T, now time.Time) (*HackerNews, *atomic.Int32) {
	var hits atomic.Int32
	old := now.AddDate(0, -2, 0).Unix()
	recent := now.AddDate(0, 0, -5).Unix()
	items := map[string]string{
		"/v0/user/whoishiring.json": `{"id":"whoishiring","submitted":[300,299,200,199]}`,
		"/v0/item/300.json":         fmt.Sprintf(`{"id":300,"type":"story","title":"Ask HN: Who is hiring? (October 2026)","time":%d,"kids":[301,302,303]}`, recent),
		"/v0/item/299.json":         `{"id":299,"type":"story","title":"Ask HN: Who wants to be hired? (October 2026)","kids":[1]}`,
		"/v0/item/200.json":         fmt.Sprintf(`{"id":200,"type":"story","title":"Ask HN: Who is hiring? (August 2026)","time":%d,"kids":[201]}`, old),
		"/v0/item/199.json":         `{"id":199,"type":"story","title":"Ask HN: Freelancer? Seeking freelancer? (August 2026)","kids":[2]}`,
		"/v0/item/301.json":         `{"id":301,"type":"comment","text":"Acme Corp | Full Stack Engineer | Remote<p>Apply: <a href=\"https:&#x2F;&#x2F;jobs.lever.co&#x2F;acme\" rel=\"nofollow\">https:&#x2F;&#x2F;jobs.lever.co&#x2F;acme</a> or see <a href=\"https:&#x2F;&#x2F;acme.example&#x2F;blog\">blog</a>"}`,
		"/v0/item/302.json":         `{"id":302,"type":"comment","text":"Beta GmbH | Berlin | <a href=\"https:&#x2F;&#x2F;beta.example&#x2F;careers\">https:&#x2F;&#x2F;beta.example&#x2F;careers</a>"}`,
		"/v0/item/303.json":         `{"id":303,"type":"comment","deleted":true}`,
		"/v0/item/201.json":         `{"id":201,"type":"comment","text":"Gamma | <a href=\"https:&#x2F;&#x2F;gamma.example\">gamma.example</a>"}`,
	}
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, ok := items[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	})
	return &HackerNews{Client: c, BaseURL: base + "/v0", Months: 12, Now: func() time.Time { return now }}, &hits
}

func TestHackerNewsDiscover(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	hn, _ := hnServer(t, now)
	got, state := collect(t, hn, "")
	if u := urls(got); u != "https://gamma.example https://jobs.lever.co/acme https://beta.example/careers" {
		t.Errorf("candidates = %s", u)
	}
	if got[1].Name != "Acme Corp" || got[1].Origin != "hackernews" || got[2].Name != "Beta GmbH" {
		t.Errorf("names = %+v", got)
	}
	// Only finished months are remembered; the current thread is read again.
	if state != "200" {
		t.Errorf("state = %q, want 200", state)
	}
}

func TestHackerNewsSkipsRememberedStories(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	hn, hits := hnServer(t, now)
	got, state := collect(t, hn, "200")
	if u := urls(got); u != "https://jobs.lever.co/acme https://beta.example/careers" {
		t.Errorf("candidates = %s", u)
	}
	if state != "200" {
		t.Errorf("state = %q", state)
	}
	if hits.Load() > 8 {
		t.Errorf("requests = %d; the old story's comments must not be read again", hits.Load())
	}
}

func TestMarkdownListDiscover(t *testing.T) {
	readme := "# Companies\n\n- [Airtable](https://airtable.com/careers) | SF | Take home\n" +
		"- [Albert](https://jobs.lever.co/meetalbert/) | LA\n* [Bad](javascript:alert(1))\n" +
		"See [the guide](https://example.com/guide) for details.\n| [Tab](https://tab.example/jobs) | x |\n"
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(readme)) })
	got, _ := collect(t, &MarkdownList{Client: c, URLs: []string{base + "/README.md"}}, "")
	if u := urls(got); u != "https://airtable.com/careers https://jobs.lever.co/meetalbert/ https://tab.example/jobs" {
		t.Errorf("candidates = %s", u)
	}
	if got[0].Name != "Airtable" || got[0].Origin != "list" {
		t.Errorf("first = %+v", got[0])
	}
}

func TestSeedsParse(t *testing.T) {
	seeds, err := ParseSeeds("# comment\n\nhttps://acme.example/careers | Acme\nhttps://jobs.lever.co/beta\n")
	if err != nil || len(seeds) != 2 || seeds[0].Name != "Acme" || seeds[1].URL != "https://jobs.lever.co/beta" {
		t.Fatalf("seeds = %+v, err = %v", seeds, err)
	}
	if _, err := ParseSeeds("https://ok.example\nnot-a-url | X\n"); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want line 2", err)
	}
}

func TestEmbeddedSeedsValid(t *testing.T) {
	seeds, err := Seeds()
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) < 100 {
		t.Errorf("only %d seeds", len(seeds))
	}
	seen := map[string]bool{}
	for _, s := range seeds {
		if seen[s.URL] {
			t.Errorf("duplicate seed %s", s.URL)
		}
		seen[s.URL] = true
	}
	got, _ := collect(t, SeedSource{}, "")
	if len(got) != len(seeds) || got[0].Origin != "seed" {
		t.Errorf("seed source returned %d candidates", len(got))
	}
}
