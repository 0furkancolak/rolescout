// Package adapters lists the source adapters this build supports and builds
// the polite HTTP client they share.
package adapters

import (
	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
	"example.com/rolescout/api/internal/ingest/ashby"
	"example.com/rolescout/api/internal/ingest/bamboohr"
	"example.com/rolescout/api/internal/ingest/greenhouse"
	"example.com/rolescout/api/internal/ingest/jsonld"
	"example.com/rolescout/api/internal/ingest/lever"
	"example.com/rolescout/api/internal/ingest/personio"
	"example.com/rolescout/api/internal/ingest/recruitee"
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
	return crawl.New(crawl.Options{
		UserAgent:         "RoleScout/" + version + " (self-hosted job search; +" + cfg.UserAgentURL + ")",
		HostInterval:      cfg.HostInterval,
		VendorHosts:       crawl.DefaultVendorHosts,
		VendorConcurrency: cfg.VendorConcurrency,
	})
}

// Registry returns every adapter. Adding an adapter requires an access review
// documented in docs/sources/ and an adapter-type review (ADR 0004).
func Registry(c *crawl.Client, o Options) ingest.Registry {
	return ingest.Registry{
		"ashby":      ashby.New(c),
		"bamboohr":   bamboohr.New(c),
		"greenhouse": greenhouse.New(c),
		"jsonld":     jsonld.New(c, o.JSONLDMaxPages),
		"lever":      lever.New(c),
		"personio":   personio.New(c),
		"recruitee":  recruitee.New(c),
		"workable":   workable.New(c),
		"workday":    workday.New(c),
	}
}
