package crawl

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(o Options) *Client {
	if o.UserAgent == "" {
		o.UserAgent = "RoleScout/test (+https://example.com)"
	}
	if o.HostInterval == 0 {
		o.HostInterval = time.Millisecond
	}
	// Test servers listen on 127.0.0.1.
	o.AllowPrivate = true
	return New(o)
}

func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func host(srv *httptest.Server) string {
	u, _ := url.Parse(srv.URL)
	return u.Host
}

func TestRobotsDisallowBlocksPath(t *testing.T) {
	var robotsHits atomic.Int32
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			robotsHits.Add(1)
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		w.Write([]byte("ok"))
	})
	c := newTestClient(Options{})
	ctx := context.Background()

	if _, err := c.Get(ctx, srv.URL+"/private/x", "", nil); !errors.Is(err, ErrDisallowed) {
		t.Fatalf("private: err = %v, want ErrDisallowed", err)
	}
	res, err := c.Get(ctx, srv.URL+"/public", "", nil)
	if err != nil || res.StatusCode != 200 || string(res.Body) != "ok" {
		t.Fatalf("public: res = %+v, err = %v", res, err)
	}
	if n := robotsHits.Load(); n != 1 {
		t.Errorf("robots.txt fetched %d times, want 1", n)
	}
}

func TestRobotsMissingAllowsAll(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("ok"))
	})
	if _, err := newTestClient(Options{}).Get(context.Background(), srv.URL+"/any", "", nil); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestRobotsServerErrorDisallowsHost(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	_, err := newTestClient(Options{}).Get(context.Background(), srv.URL+"/any", "", nil)
	if !errors.Is(err, ErrDisallowed) {
		t.Fatalf("err = %v, want ErrDisallowed", err)
	}
}

func TestHostIntervalSpacesRequests(t *testing.T) {
	a := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("a")) })
	b := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("b")) })
	c := newTestClient(Options{HostInterval: 100 * time.Millisecond})
	ctx := context.Background()
	// Warm the robots cache so only the measured requests count.
	c.Get(ctx, a.URL+"/", "", nil)
	c.Get(ctx, b.URL+"/", "", nil)
	time.Sleep(110 * time.Millisecond)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Get(ctx, a.URL+"/x", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Errorf("3 requests to one host took %v, want at least 200ms", d)
	}

	time.Sleep(110 * time.Millisecond)
	start = time.Now()
	var wg sync.WaitGroup
	for _, u := range []string{a.URL, b.URL} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get(ctx, u+"/y", "", nil)
		}()
	}
	wg.Wait()
	if d := time.Since(start); d > 90*time.Millisecond {
		t.Errorf("requests to two hosts took %v, want them in parallel", d)
	}
}

func TestVendorHostAllowsConcurrency(t *testing.T) {
	var current, peak atomic.Int32
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			return
		}
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		current.Add(-1)
	})
	c := newTestClient(Options{VendorHosts: []string{host(srv)}, VendorConcurrency: 4, VendorInterval: time.Millisecond, HostInterval: time.Second})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Get(context.Background(), srv.URL+"/jobs", "", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if p := peak.Load(); p != 4 {
		t.Errorf("peak concurrency = %d, want 4", p)
	}
}

func TestVendorSuffixMatchesSubdomains(t *testing.T) {
	c := newTestClient(Options{VendorHosts: []string{".recruitee.com", "api.lever.co"}})
	for h, want := range map[string]string{
		"acme.recruitee.com":  ".recruitee.com",
		"other.recruitee.com": ".recruitee.com",
		"api.lever.co":        "api.lever.co",
		"jobs.lever.co":       "",
		"recruitee.com.evil":  "",
	} {
		if got := c.vendorKey(h); got != want {
			t.Errorf("vendorKey(%q) = %q, want %q", h, got, want)
		}
	}
}

func TestRetryAfterReturnedOn429(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
		case "/busy":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	})
	c := newTestClient(Options{})
	_, err := c.Get(context.Background(), srv.URL+"/busy", "", nil)
	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != 429 || se.RetryAfter != 7*time.Second {
		t.Fatalf("err = %#v, want StatusError 429 with 7s", err)
	}
	if !IsTemporary(err) {
		t.Error("429 should be temporary")
	}
	_, err = c.Get(context.Background(), srv.URL+"/missing", "", nil)
	if !errors.As(err, &se) || se.StatusCode != 404 || IsTemporary(err) {
		t.Fatalf("404: err = %v, temporary = %v", err, IsTemporary(err))
	}
}

func TestBodyLimit(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			w.Write([]byte(strings.Repeat("x", 100)))
		}
	})
	_, err := newTestClient(Options{MaxBodyBytes: 10}).Get(context.Background(), srv.URL+"/big", "", nil)
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("err = %v, want size error", err)
	}
}

func TestUserAgentAndAcceptSent(t *testing.T) {
	var ua, accept string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			ua, accept = r.UserAgent(), r.Header.Get("Accept")
		}
	})
	c := newTestClient(Options{UserAgent: "RoleScout/1.0 (+https://example.com)"})
	if _, err := c.Get(context.Background(), srv.URL+"/", "application/json", nil); err != nil {
		t.Fatal(err)
	}
	if ua != "RoleScout/1.0 (+https://example.com)" || accept != "application/json" {
		t.Errorf("User-Agent = %q, Accept = %q", ua, accept)
	}
}

func TestRedirectFollowedWithFinalURL(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
		case "/old":
			http.Redirect(w, r, "/new", http.StatusFound)
		case "/new":
			w.Write([]byte("moved"))
		case "/ftp":
			http.Redirect(w, r, "ftp://example.com/file", http.StatusFound)
		}
	})
	c := newTestClient(Options{})
	res, err := c.Get(context.Background(), srv.URL+"/old", "", nil)
	if err != nil || res.FinalURL != srv.URL+"/new" || string(res.Body) != "moved" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, err := c.Get(context.Background(), srv.URL+"/ftp", "", nil); err == nil {
		t.Fatal("redirect to ftp should fail")
	}
}

func TestRedirectChecksRobotsOfTarget(t *testing.T) {
	target := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /\n"))
		}
	})
	origin := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/go" {
			http.Redirect(w, r, target.URL+"/page", http.StatusFound)
		}
	})
	_, err := newTestClient(Options{}).Get(context.Background(), origin.URL+"/go", "", nil)
	if !errors.Is(err, ErrDisallowed) {
		t.Fatalf("err = %v, want ErrDisallowed", err)
	}
}

func TestConditionalRequest304(t *testing.T) {
	var hits atomic.Int32
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			return
		}
		hits.Add(1)
		if r.Header.Get(conditionalHeader) != "" {
			t.Error("internal header leaked to the server")
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Write([]byte("body"))
	})
	c := newTestClient(Options{})
	cond := http.Header{conditionalHeader: {"1"}}
	res, err := c.Get(context.Background(), srv.URL+"/page", "", cond)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("first: %+v %v", res, err)
	}
	res, err = c.Get(context.Background(), srv.URL+"/page", "", cond)
	if err != nil || res.StatusCode != http.StatusNotModified {
		t.Fatalf("second: %+v %v", res, err)
	}
	// Without the conditional marker the validator is not sent.
	res, err = c.Get(context.Background(), srv.URL+"/page", "", nil)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("unconditional: %+v %v", res, err)
	}
}

func TestNetworkErrorIsTemporary(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {})
	addr := srv.URL
	srv.Close()
	_, err := newTestClient(Options{}).Get(context.Background(), addr+"/x", "", nil)
	if err == nil || !IsTemporary(err) {
		t.Fatalf("err = %v, temporary = %v", err, IsTemporary(err))
	}
}

func TestDeadlineIsTemporaryButCancelIsNot(t *testing.T) {
	if !IsTemporary(context.DeadlineExceeded) {
		t.Error("an expired deadline may pass on a later attempt")
	}
	if IsTemporary(context.Canceled) {
		t.Error("cancellation is final")
	}
}

func TestCrawlDelayIsHonoredForVendorHosts(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nAllow: /\nCrawl-delay: 0.1\n"))
			return
		}
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
	})
	c := newTestClient(Options{VendorHosts: []string{host(srv)}, VendorConcurrency: 4, VendorInterval: time.Millisecond})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Get(context.Background(), srv.URL+"/x", "", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(starts) != 4 {
		t.Fatalf("requests = %d", len(starts))
	}
	sortTimes(starts)
	for i := 1; i < len(starts); i++ {
		// Arrival times jitter on a busy machine, so allow half the delay; without the
		// delay the four requests would arrive within a millisecond or two.
		if gap := starts[i].Sub(starts[i-1]); gap < 50*time.Millisecond {
			t.Errorf("gap %d = %v, want about the 100ms crawl delay", i, gap)
		}
	}
}

func sortTimes(ts []time.Time) {
	for i := range ts {
		for j := i + 1; j < len(ts); j++ {
			if ts[j].Before(ts[i]) {
				ts[i], ts[j] = ts[j], ts[i]
			}
		}
	}
}

func TestPrivateAddressesAreRefused(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("internal")) })
	c := New(Options{UserAgent: "RoleScout/test", HostInterval: time.Millisecond})
	for _, u := range []string{srv.URL + "/x", "http://localhost:1/x", "http://10.0.0.1:1/x", "http://169.254.169.254/latest", "http://[::1]:1/"} {
		_, err := c.Get(context.Background(), u, "", nil)
		if !errors.Is(err, ErrPrivateAddress) {
			t.Errorf("%s: err = %v, want ErrPrivateAddress", u, err)
		}
		if IsTemporary(err) {
			t.Errorf("%s: a refused address must not be retried", u)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	if IsPublicIP(net.ParseIP("127.0.0.1")) || !IsPublicIP(net.ParseIP("93.184.216.34")) || IsPublicIP(net.ParseIP("192.168.1.1")) || IsPublicIP(net.ParseIP("fd00::1")) {
		t.Fatal("isPublicIP misclassifies addresses")
	}
}
