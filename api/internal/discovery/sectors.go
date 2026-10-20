package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"example.com/rolescout/api/internal/focus"
	"example.com/rolescout/api/internal/llm"
)

// Sectors are the industries the sector feed walks, one per discovery run
// batch. Software engineers are hired in all of them.
var Sectors = []string{
	"fintech and payments", "banking and insurance", "e-commerce and marketplaces", "gaming", "healthtech and biotech",
	"telecommunications", "logistics and supply chain", "mobility and automotive software", "energy and cleantech",
	"edtech", "adtech and marketing technology", "cybersecurity", "developer tools and infrastructure software",
	"cloud and data platforms", "artificial intelligence", "enterprise software (SaaS)", "travel and hospitality technology",
	"media and streaming", "defence and aerospace", "retail and consumer brands", "food delivery and grocery technology",
	"real estate technology", "government and civic technology", "semiconductors and hardware", "consulting and IT services",
}

// SectorRegions are the places each sector is asked about besides the research
// profile's own countries.
var SectorRegions = []string{"worldwide", "Türkiye", "Europe", "United States"}

// SectorFeed asks the AI for companies of one sector and region at a time. The
// names are only candidates: FeedSource keeps those whose job board really
// exists, so a name the model got wrong costs a few requests and nothing else.
type SectorFeed struct {
	LLM llm.Completer
	// Enabled reports whether AI is on; while it is off the feed is skipped.
	Enabled func(ctx context.Context) bool
	// PerRun is how many sector/region pairs are asked per run (default 4).
	PerRun int
}

func (s *SectorFeed) Name() string { return "sectors" }

// Ready is true while AI is switched on.
func (s *SectorFeed) Ready(ctx context.Context) bool { return s.Enabled == nil || s.Enabled(ctx) }

const sectorPrompt = `You list real companies for a job search. Reply with JSON: {"companies": ["..."]}. Rules: at most 40 names; only companies you are sure exist; the name people use, without "Inc" or "Ltd"; companies that employ software engineers; no invented names; an empty list when unsure.`

var sectorSchema = json.RawMessage(`{"type":"object","properties":{"companies":{"type":"array","items":{"type":"string"},"maxItems":40}},"required":["companies"],"additionalProperties":false}`)

// regionsFor lists the places to ask about: the research profile's countries
// first, then the general regions. key identifies that list, so progress made
// for one profile is not mistaken for progress through another's.
func regionsFor(ctx context.Context) (regions []string, key string) {
	codes := focus.Countries(ctx)
	for _, c := range codes {
		name := CountryNames[c]
		if name == "" {
			name = c
		}
		regions = append(regions, name)
	}
	for _, r := range SectorRegions {
		if !slices.Contains(regions, r) {
			regions = append(regions, r)
		}
	}
	return regions, strings.Join(codes, ",")
}

func (s *SectorFeed) Next(ctx context.Context, state string) ([]string, string, bool, error) {
	regions, key := regionsFor(ctx)
	total := len(Sectors) * len(regions)
	at := 0
	if k, n, ok := strings.Cut(state, ":"); ok && k == key {
		at, _ = strconv.Atoi(n)
	}
	per := s.PerRun
	if per <= 0 {
		per = 4
	}
	end := min(at+per, total)
	// Every sector of one region is asked before the next region, so a country
	// the profile focuses on is covered completely first. The questions are
	// independent, so they are asked at the same time.
	results := make([][]string, end-at)
	errs := make([]error, end-at)
	var wg sync.WaitGroup
	for i := at; i < end; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sector, region := Sectors[i%len(Sectors)], regions[i/len(Sectors)]
			results[i-at], errs[i-at] = s.ask(ctx, sector, region)
		}()
	}
	wg.Wait()
	var names []string
	seen := map[string]bool{}
	for i := range results {
		if errs[i] != nil {
			return nil, state, false, errs[i]
		}
		for _, n := range results[i] {
			if k := strings.ToLower(n); !seen[k] {
				seen[k] = true
				names = append(names, n)
			}
		}
	}
	return names, key + ":" + strconv.Itoa(end), end < total, nil
}

// ask returns the companies the model names for one sector and region. An
// answer that is not usable gives no names rather than an error.
func (s *SectorFeed) ask(ctx context.Context, sector, region string) ([]string, error) {
	resp, err := s.LLM.Complete(ctx, llm.Request{
		System: sectorPrompt, User: fmt.Sprintf("Industry: %s\nRegion: %s", sector, region),
		Schema: sectorSchema, MaxTokens: 700,
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Companies []string `json:"companies"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out) != nil {
		return nil, nil
	}
	var names []string
	for _, n := range out.Companies {
		if n = strings.Join(strings.Fields(n), " "); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}
