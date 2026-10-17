// Package crawl is the HTTP client for everything RoleScout fetches from other
// sites. It identifies itself, follows robots.txt, limits how fast it calls
// each host and bounds response sizes, so discovery and ingestion stay polite.
package crawl

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrDisallowed is returned when robots.txt does not allow a URL.
var ErrDisallowed = errors.New("disallowed by robots.txt")

// DefaultVendorHosts are job board API hosts that serve many companies.
// Entries starting with "." match every subdomain and share one limit.
var DefaultVendorHosts = []string{
	"boards-api.greenhouse.io", "api.lever.co", "api.eu.lever.co", "api.ashbyhq.com",
	"apply.workable.com", "api.smartrecruiters.com",
	".recruitee.com", ".jobs.personio.de", ".jobs.personio.com", ".bamboohr.com", ".myworkdayjobs.com",
}

// conditionalHeader asks Get to send stored validators (ETag,
// Last-Modified) for the URL. It is removed before the request is sent.
const conditionalHeader = "X-Rolescout-Conditional" // canonical form

// Conditional is an extra header set that makes Get a conditional request.
func Conditional() http.Header { return http.Header{conditionalHeader: {"1"}} }

const (
	maxRedirects    = 5
	validatorsLimit = 10_000
	robotsBodyLimit = 512 << 10
)

type Options struct {
	// UserAgent identifies RoleScout, such as "RoleScout/1.0 (+https://…)".
	// Its first product token is matched against robots.txt groups.
	UserAgent string
	// HostInterval spaces requests to the same ordinary host (default 1s).
	HostInterval time.Duration
	// VendorHosts get VendorConcurrency parallel requests spaced by
	// VendorInterval (defaults 4 and 250ms).
	VendorHosts       []string
	VendorConcurrency int
	VendorInterval    time.Duration
	Timeout           time.Duration // per request, default 60s
	MaxBodyBytes      int64         // default 32 MiB
	RobotsTTL         time.Duration // default 24h
	Transport         http.RoundTripper
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	FinalURL   string
}

// StatusError is a response with a status other than 2xx or 304.
type StatusError struct {
	URL        string
	StatusCode int
	RetryAfter time.Duration
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s responded %d %s", e.URL, e.StatusCode, http.StatusText(e.StatusCode))
}

type temporaryError struct{ err error }

func (e temporaryError) Error() string { return e.err.Error() }
func (e temporaryError) Unwrap() error { return e.err }

// IsTemporary reports whether a later attempt may succeed: network errors,
// timeouts (including an expired request context), 429 and 5xx responses.
func IsTemporary(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode == http.StatusTooManyRequests || se.StatusCode >= 500
	}
	var te temporaryError
	return errors.As(err, &te)
}

type Client struct {
	o     Options
	http  *http.Client
	agent string

	mu         sync.Mutex
	limiters   map[string]*hostLimiter
	robots     map[string]robotsEntry
	robotsMu   map[string]*sync.Mutex
	validators map[string]*list.Element
	order      *list.List
}

type robotsEntry struct {
	rules   robotsRules
	expires time.Time
}

type validator struct {
	url, etag, lastModified string
}

func New(o Options) *Client {
	if o.HostInterval <= 0 {
		o.HostInterval = time.Second
	}
	if o.VendorConcurrency <= 0 {
		o.VendorConcurrency = 4
	}
	if o.VendorInterval <= 0 {
		o.VendorInterval = 250 * time.Millisecond
	}
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = 32 << 20
	}
	if o.RobotsTTL <= 0 {
		o.RobotsTTL = 24 * time.Hour
	}
	agent, _, _ := strings.Cut(o.UserAgent, "/")
	agent, _, _ = strings.Cut(agent, " ")
	return &Client{
		o:     o,
		agent: agent,
		http: &http.Client{
			Timeout:   o.Timeout,
			Transport: o.Transport,
			// Redirects are followed in Get so every hop is checked.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		limiters:   map[string]*hostLimiter{},
		robots:     map[string]robotsEntry{},
		robotsMu:   map[string]*sync.Mutex{},
		validators: map[string]*list.Element{},
		order:      list.New(),
	}
}

// vendorKey returns the vendor entry host matches, or "".
func (c *Client) vendorKey(host string) string {
	host = strings.ToLower(host)
	for _, v := range c.o.VendorHosts {
		if strings.HasPrefix(v, ".") {
			if strings.HasSuffix(host, v) {
				return v
			}
		} else if host == v {
			return v
		}
	}
	return ""
}

func (c *Client) limiter(host string) *hostLimiter {
	key, concurrency, interval := host, 1, c.o.HostInterval
	if v := c.vendorKey(host); v != "" {
		key, concurrency, interval = v, c.o.VendorConcurrency, c.o.VendorInterval
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.limiters[key]
	if !ok {
		l = newHostLimiter(concurrency, interval)
		c.limiters[key] = l
	}
	return l
}

// Get fetches rawURL, following up to 5 redirects. accept sets the Accept
// header when not empty; extra headers are added to every hop.
func (c *Client) Get(ctx context.Context, rawURL, accept string, extra http.Header) (*Response, error) {
	return c.do(ctx, http.MethodGet, rawURL, accept, "", nil, extra)
}

// Post sends body to rawURL with the same robots.txt and rate limits as Get.
func (c *Client) Post(ctx context.Context, rawURL, contentType string, body []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, rawURL, "application/json", contentType, body, nil)
}

func (c *Client) do(ctx context.Context, method, rawURL, accept, contentType string, body []byte, extra http.Header) (*Response, error) {
	conditional := extra.Get(conditionalHeader) != ""
	current := rawURL
	for hop := 0; ; hop++ {
		u, err := url.Parse(current)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("invalid URL %q: only absolute http(s) URLs can be fetched", current)
		}
		if err := c.checkRobots(ctx, u); err != nil {
			return nil, err
		}
		res, err := c.send(ctx, method, u, accept, contentType, body, extra, conditional)
		if err != nil {
			return nil, err
		}
		if res.StatusCode >= 300 && res.StatusCode < 400 && res.StatusCode != http.StatusNotModified {
			loc := res.Header.Get("Location")
			if loc == "" {
				return nil, fmt.Errorf("%s redirected without a location", current)
			}
			if hop >= maxRedirects {
				return nil, fmt.Errorf("%s: too many redirects", rawURL)
			}
			next, err := u.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("%s redirected to an invalid location: %w", current, err)
			}
			current = next.String()
			if res.StatusCode == http.StatusSeeOther {
				method, body, contentType = http.MethodGet, nil, ""
			}
			continue
		}
		res.FinalURL = current
		if res.StatusCode != http.StatusNotModified && (res.StatusCode < 200 || res.StatusCode > 299) {
			return res, &StatusError{URL: current, StatusCode: res.StatusCode, RetryAfter: retryAfter(res.Header.Get("Retry-After"))}
		}
		if conditional && res.StatusCode == http.StatusOK {
			c.storeValidator(current, res.Header)
		}
		return res, nil
	}
}

func (c *Client) send(ctx context.Context, method string, u *url.URL, accept, contentType string, body []byte, extra http.Header, conditional bool) (*Response, error) {
	release, err := c.limiter(u.Host).acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	for k, vs := range extra {
		if http.CanonicalHeaderKey(k) == conditionalHeader {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("User-Agent", c.o.UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if conditional && method == http.MethodGet {
		if v, ok := c.validator(u.String()); ok {
			if v.etag != "" {
				req.Header.Set("If-None-Match", v.etag)
			}
			if v.lastModified != "" {
				req.Header.Set("If-Modified-Since", v.lastModified)
			}
		}
	}

	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, temporaryError{err}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, c.o.MaxBodyBytes+1))
	if err != nil {
		return nil, temporaryError{fmt.Errorf("read %s: %w", u.Redacted(), err)}
	}
	if int64(len(data)) > c.o.MaxBodyBytes {
		return nil, fmt.Errorf("%s: response is larger than %d bytes", u.Redacted(), c.o.MaxBodyBytes)
	}
	return &Response{StatusCode: res.StatusCode, Header: res.Header, Body: data}, nil
}

// checkRobots fetches and caches robots.txt for u's origin and checks u.
func (c *Client) checkRobots(ctx context.Context, u *url.URL) error {
	if u.Path == "/robots.txt" {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	rules, err := c.robotsFor(ctx, origin)
	if err != nil {
		return err
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if !rules.allowed(path) {
		return fmt.Errorf("%s: %w", u.Redacted(), ErrDisallowed)
	}
	return nil
}

func (c *Client) robotsFor(ctx context.Context, origin string) (robotsRules, error) {
	c.mu.Lock()
	lock, ok := c.robotsMu[origin]
	if !ok {
		lock = &sync.Mutex{}
		c.robotsMu[origin] = lock
	}
	c.mu.Unlock()

	// One fetch per origin at a time; others wait for its result.
	lock.Lock()
	defer lock.Unlock()
	c.mu.Lock()
	entry, ok := c.robots[origin]
	c.mu.Unlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.rules, nil
	}

	u, _ := url.Parse(origin + "/robots.txt")
	res, err := c.send(ctx, http.MethodGet, u, "text/plain", "", nil, nil, false)
	var rules robotsRules
	switch {
	case err != nil && ctx.Err() != nil:
		return rules, ctx.Err()
	case err != nil:
		// Unreachable robots.txt: do not cache, let the caller retry later.
		return rules, err
	case res.StatusCode >= 200 && res.StatusCode < 300:
		body := res.Body
		if len(body) > robotsBodyLimit {
			body = body[:robotsBodyLimit]
		}
		rules = parseRobots(body, c.agent)
	case res.StatusCode >= 500:
		rules = robotsRules{disallowAll: true}
	default:
		// 3xx (not followed) and 4xx: no restrictions (RFC 9309, 2.3.1).
	}
	c.mu.Lock()
	c.robots[origin] = robotsEntry{rules: rules, expires: time.Now().Add(c.o.RobotsTTL)}
	c.mu.Unlock()
	return rules, nil
}

func (c *Client) validator(u string) (validator, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.validators[u]
	if !ok {
		return validator{}, false
	}
	c.order.MoveToFront(el)
	return el.Value.(validator), true
}

func (c *Client) storeValidator(u string, h http.Header) {
	v := validator{url: u, etag: h.Get("ETag"), lastModified: h.Get("Last-Modified")}
	if v.etag == "" && v.lastModified == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.validators[u]; ok {
		el.Value = v
		c.order.MoveToFront(el)
		return
	}
	c.validators[u] = c.order.PushFront(v)
	if c.order.Len() > validatorsLimit {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.validators, oldest.Value.(validator).url)
	}
}

func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
