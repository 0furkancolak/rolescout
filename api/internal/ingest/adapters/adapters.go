// Package adapters lists the source adapters this build supports and builds
// the polite HTTP client they share.
package adapters

import (
	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/arbeitnow"
	"example.com/rolescout/api/internal/ingest/ashby"
	"example.com/rolescout/api/internal/ingest/bamboohr"
	"example.com/rolescout/api/internal/ingest/greenhouse"
	"example.com/rolescout/api/internal/ingest/himalayas"
	"example.com/rolescout/api/internal/ingest/hnhiring"
	"example.com/rolescout/api/internal/ingest/htmlpage"
	"example.com/rolescout/api/internal/ingest/jobicy"
	"example.com/rolescout/api/internal/ingest/jsonld"
	"example.com/rolescout/api/internal/ingest/kariyernet"
	"example.com/rolescout/api/internal/ingest/lever"
	"example.com/rolescout/api/internal/ingest/personio"
	"example.com/rolescout/api/internal/ingest/recruitee"
	"example.com/rolescout/api/internal/ingest/remoteok"
	"example.com/rolescout/api/internal/ingest/themuse"
	"example.com/rolescout/api/internal/ingest/weworkremotely"
	"example.com/rolescout/api/internal/ingest/workable"
	"example.com/rolescout/api/internal/ingest/workday"
	"example.com/rolescout/api/internal/ingest/workingnomads"
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
		"arbeitnow":      arbeitnow.New(c),
		"ashby":          ashby.New(c),
		"bamboohr":       bamboohr.New(c),
		"greenhouse":     greenhouse.New(c),
		"himalayas":      himalayas.New(c),
		"hnhiring":       hnhiring.New(c),
		"htmlpage":       htmlpage.New(c),
		"jobicy":         jobicy.New(c),
		"jsonld":         jsonld.New(c, o.JSONLDMaxPages),
		"kariyernet":     kariyernet.New(c),
		"lever":          lever.New(c),
		"personio":       personio.New(c),
		"recruitee":      recruitee.New(c),
		"remoteok":       remoteok.New(c),
		"themuse":        themuse.New(c),
		"weworkremotely": weworkremotely.New(c),
		"workable":       workable.New(c),
		"workday":        workday.New(c),
		"workingnomads":  workingnomads.New(c),
	}
}
