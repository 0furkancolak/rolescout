package discovery

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// ExchangeURLs are Nasdaq Trader's public symbol directories: every company
// listed on Nasdaq, NYSE, NYSE American and other US exchanges.
var ExchangeURLs = []string{
	"https://www.nasdaqtrader.com/dynamic/symdir/nasdaqlisted.txt",
	"https://www.nasdaqtrader.com/dynamic/symdir/otherlisted.txt",
}

// ExchangeFeed offers the names of listed companies. Listed companies are
// large employers that mostly keep their jobs on a known job board system.
type ExchangeFeed struct {
	Client *crawl.Client
	URLs   []string // default ExchangeURLs
	// Batch is how many names are handed out per discovery run (default 150).
	Batch int
}

func (e *ExchangeFeed) Name() string { return "exchange" }

func (e *ExchangeFeed) Next(ctx context.Context, state string) ([]string, string, bool, error) {
	urls := e.URLs
	if len(urls) == 0 {
		urls = ExchangeURLs
	}
	var names []string
	seen := map[string]bool{}
	for _, u := range urls {
		body, err := ingest.Fetch(ctx, e.Client, u, "text/plain")
		if err != nil {
			return nil, state, false, fmt.Errorf("read %s: %w", u, err)
		}
		for _, n := range ParseListing(string(body)) {
			if k := strings.ToLower(n); !seen[k] {
				seen[k] = true
				names = append(names, n)
			}
		}
	}
	start, _ := strconv.Atoi(state)
	batch := e.Batch
	if batch <= 0 {
		batch = 150
	}
	if start >= len(names) {
		return nil, "", false, nil
	}
	end := min(start+batch, len(names))
	return names[start:end], strconv.Itoa(end), end < len(names), nil
}

var notOperatingCompany = []string{
	"warrant", " right", " units", " unit", "preferred", "notes due", "depositary", " trust", " fund", " etf", "acquisition",
	"% ", "debenture", "subordinated", "senior notes", "bond", "index", "ishares", "proshares", "spdr",
}

// ParseListing reads a Nasdaq Trader directory: pipe-separated rows with a
// header and a trailing "File Creation Time" line. Test issues, ETFs and
// securities that are not a company's shares are skipped.
func ParseListing(text string) []string {
	var out []string
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	header := strings.Split(strings.TrimSpace(lines[0]), "|")
	col := func(name string) int {
		for i, h := range header {
			if strings.EqualFold(h, name) {
				return i
			}
		}
		return -1
	}
	nameCol, testCol, etfCol := col("Security Name"), col("Test Issue"), col("ETF")
	if nameCol < 0 {
		return nil
	}
	for _, line := range lines[1:] {
		f := strings.Split(strings.TrimSpace(line), "|")
		if len(f) <= nameCol || strings.HasPrefix(line, "File Creation Time") {
			continue
		}
		if (testCol >= 0 && len(f) > testCol && f[testCol] == "Y") || (etfCol >= 0 && len(f) > etfCol && f[etfCol] == "Y") {
			continue
		}
		security := f[nameCol]
		low := strings.ToLower(security)
		skip := false
		for _, bad := range notOperatingCompany {
			if strings.Contains(low, bad) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if n := cleanCompanyName(security); n != "" {
			out = append(out, n)
		}
	}
	return out
}
