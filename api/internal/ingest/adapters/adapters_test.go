package adapters

import (
	"strings"
	"testing"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
)

func TestRegistryHasReviewedAdapters(t *testing.T) {
	r := Registry(crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "RoleScout/test"}), Options{JSONLDMaxPages: 10})
	names := strings.Join(r.Names(), ",")
	if names != "arbeitnow,ashby,bamboohr,greenhouse,himalayas,hnhiring,htmlpage,jobicy,jsonld,kariyernet,lever,personio,recruitee,remoteok,themuse,weworkremotely,workable,workday,workingnomads" {
		t.Fatalf("registry = %s", names)
	}
}

func TestNewCrawlerUserAgent(t *testing.T) {
	c := NewCrawler("1.2.3", config.Crawl{UserAgentURL: "https://example.com/bot", HostInterval: 1, VendorConcurrency: 2})
	if c == nil {
		t.Fatal("nil crawler")
	}
}
