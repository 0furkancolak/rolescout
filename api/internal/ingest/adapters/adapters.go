// Package adapters lists the source adapters this build supports and builds
// the polite HTTP client they share.
package adapters

import (
	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/ashby"
	"example.com/rolescout/api/internal/ingest/bamboohr"
	"example.com/rolescout/api/internal/ingest/breezy"
	"example.com/rolescout/api/internal/ingest/gem"
	"example.com/rolescout/api/internal/ingest/greenhouse"
	"example.com/rolescout/api/internal/ingest/htmlpage"
	"example.com/rolescout/api/internal/ingest/jsonld"
	"example.com/rolescout/api/internal/ingest/kariyernet"
	"example.com/rolescout/api/internal/ingest/lever"
	"example.com/rolescout/api/internal/ingest/personio"
	"example.com/rolescout/api/internal/ingest/recruitee"
	"example.com/rolescout/api/internal/ingest/rippling"
	"example.com/rolescout/api/internal/ingest/teamtailor"
	"example.com/rolescout/api/internal/ingest/workable"
	"example.com/rolescout/api/internal/ingest/workday"
)

// Options configures adapters that need more than the HTTP client.
type Options struct {
	// JSONLDMaxPages bounds the pages read per career site by "jsonld".
	JSONLDMaxPages int
}

// NewCrawler returns the HTTP client for discovery and ingestion, identified
// as RoleScout/<version>.
func NewCrawler(version string, cfg config.Crawl) *crawl.Client {
	return NewCrawlerLive(version, cfg, nil)
}

// NewCrawlerLive is NewCrawler with a pace that can change while it runs.
func NewCrawlerLive(version string, cfg config.Crawl, live *crawl.Live) *crawl.Client {
	return crawl.New(crawl.Options{
		UserAgent:         "RoleScout/" + version + " (self-hosted job search; +" + cfg.UserAgentURL + ")",
		HostInterval:      cfg.HostInterval,
		VendorHosts:       crawl.DefaultVendorHosts,
		VendorConcurrency: cfg.VendorConcurrency,
		VendorInterval:    cfg.VendorInterval,
		Live:              live,
		AllowPrivate:      cfg.AllowPrivateAddresses,
	})
}

// Registry returns every adapter. Adding an adapter requires an access review
// documented in docs/sources/ and an adapter-type review (ADR 0004).
func Registry(c *crawl.Client, o Options) ingest.Registry {
	return ingest.Registry{
		"ashby":      ashby.New(c),
		"bamboohr":   bamboohr.New(c),
		"breezy":     breezy.New(c),
		"gem":        gem.New(c),
		"greenhouse": greenhouse.New(c),
		"htmlpage":   htmlpage.New(c),
		"jsonld":     jsonld.New(c, o.JSONLDMaxPages),
		"kariyernet": kariyernet.New(c),
		"lever":      lever.New(c),
		"personio":   personio.New(c),
		"recruitee":  recruitee.New(c),
		"rippling":   rippling.New(c),
		"teamtailor": teamtailor.New(c),
		"workable":   workable.New(c),
		"workday":    workday.New(c),
	}
}
