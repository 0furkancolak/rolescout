package adapters

import (
	"strings"
	"testing"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/crawl"
)

func TestRegistryHasReviewedAdapters(t *testing.T) {
	r := Registry(crawl.New(crawl.Options{UserAgent: "RoleScout/test"}), Options{JSONLDMaxPages: 10})
	names := strings.Join(r.Names(), ",")
	if names != "ashby,bamboohr,greenhouse,lever,personio,recruitee,workable,workday" {
		t.Fatalf("registry = %s", names)
	}
}

func TestNewCrawlerUserAgent(t *testing.T) {
	c := NewCrawler("1.2.3", config.Crawl{UserAgentURL: "https://example.com/bot", HostInterval: 1, VendorConcurrency: 2})
	if c == nil {
		t.Fatal("nil crawler")
	}
}
