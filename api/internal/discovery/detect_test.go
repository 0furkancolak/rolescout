package discovery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

// rewrite sends every request to target while keeping the original Host, so
// one test server can play several sites.
type rewrite struct{ target *url.URL }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Host = req.URL.Host
	out.URL.Scheme, out.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(out)
}

// sites serves pages keyed by host+path; robots.txt is "allow all" unless
// pages has a robots entry.
func sites(t *testing.T, pages map[string]string) *Detector {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Host + r.URL.Path
		if r.URL.Path == "/robots.txt" {
			if body, ok := pages[key]; ok {
				w.Write([]byte(body))
				return
			}
			http.NotFound(w, r)
			return
		}
		if loc, ok := pages["redirect:"+key]; ok {
			http.Redirect(w, r, loc, http.StatusFound)
			return
		}
		body, ok := pages[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	c := crawl.New(crawl.Options{UserAgent: "RoleScout/test", HostInterval: time.Millisecond, Transport: rewrite{target}})
	return &Detector{Client: c}
}

func TestDetectKnownBoardURLNeedsNoRequest(t *testing.T) {
	d := sites(t, nil)
	got, err := d.Detect(ingesttest.Ctx(t), "https://jobs.lever.co/acme")
	if err != nil || got.Board.ATS != "lever" || got.Board.ID != "acme" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
}

func TestDetectFollowsRedirectToATS(t *testing.T) {
	d := sites(t, map[string]string{
		"redirect:acme.example/careers": "https://boards.greenhouse.io/acme",
		"boards.greenhouse.io/acme":     "<html>board</html>",
	})
	got, err := d.Detect(ingesttest.Ctx(t), "http://acme.example/careers")
	if err != nil || got.Board.ATS != "greenhouse" || got.Board.ID != "acme" || got.FinalURL != "https://boards.greenhouse.io/acme" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
}

func TestDetectEmbedScript(t *testing.T) {
	d := sites(t, map[string]string{"acme.example/careers": `<html><head><title>Careers at Acme</title>
		<meta property="og:site_name" content="Acme"></head><body>
		<div id="grnhse_app"></div>
		<script src="https://boards.greenhouse.io/embed/job_board/js?for=acmeinc"></script>
		<a href="https://jobs.lever.co/acme-labs">Labs jobs</a></body></html>`})
	got, err := d.Detect(ingesttest.Ctx(t), "http://acme.example/careers")
	if err != nil || got.Board.ATS != "greenhouse" || got.Board.ID != "acmeinc" || got.Name != "Acme" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
	if len(got.Links) != 1 || got.Links[0].ATS != "lever" || got.Links[0].ID != "acme-labs" {
		t.Errorf("links = %+v", got.Links)
	}
}

func TestDetectIframeLever(t *testing.T) {
	d := sites(t, map[string]string{"acme.example/jobs": `<html><title> Jobs | Acme </title><iframe src="https://jobs.lever.co/acme?embed=true"></iframe></html>`})
	got, err := d.Detect(ingesttest.Ctx(t), "http://acme.example/jobs")
	if err != nil || got.Board.ATS != "lever" || got.Name != "Jobs | Acme" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
}

func TestDetectJSONLD(t *testing.T) {
	d := sites(t, map[string]string{
		"acme.example/careers":     `<html><a href="/careers/dev">Dev</a></html>`,
		"acme.example/careers/dev": `<html><script type="application/ld+json">{"@type":"JobPosting","title":"Dev"}</script></html>`,
		"other.example/careers":    `<html><script type="application/ld+json">{"@type":"JobPosting","title":"Ops"}</script></html>`,
	})
	got, err := d.Detect(ingesttest.Ctx(t), "http://other.example/careers")
	if err != nil || got.Board.ATS != "jsonld" || got.Board.Config["url"] != "http://other.example/careers" {
		t.Fatalf("index with JSON-LD: %+v, err = %v", got, err)
	}
	// JSON-LD only on a linked job page still identifies the site.
	got, err = d.Detect(ingesttest.Ctx(t), "http://acme.example/careers")
	if err != nil || got.Board.ATS != "jsonld" || got.Board.Config["url"] != "http://acme.example/careers" {
		t.Fatalf("linked JSON-LD: %+v, err = %v", got, err)
	}
}

func TestDetectUnknown(t *testing.T) {
	d := sites(t, map[string]string{"acme.example/careers": `<html><p>Mail us your CV</p></html>`})
	got, err := d.Detect(ingesttest.Ctx(t), "http://acme.example/careers")
	if err != nil || got.Board.ATS != "unknown" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
}

func TestDetectRobotsBlocked(t *testing.T) {
	d := sites(t, map[string]string{
		"acme.example/robots.txt": "User-agent: *\nDisallow: /careers\n",
		"acme.example/careers":    "<html></html>",
	})
	_, err := d.Detect(ingesttest.Ctx(t), "http://acme.example/careers")
	if !errors.Is(err, crawl.ErrDisallowed) {
		t.Fatalf("err = %v, want ErrDisallowed", err)
	}
}
