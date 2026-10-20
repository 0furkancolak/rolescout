package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

// SectorRegions are the places each sector is asked about.
var SectorRegions = []string{"worldwide", "Türkiye", "Europe", "United States"}

// SectorFeed asks the AI for companies of one sector and region at a time. The
// names are only candidates: FeedSource keeps those whose job board really
// exists, so a name the model got wrong costs a few requests and nothing else.
type SectorFeed struct {
	LLM llm.Completer
	// Enabled reports whether AI is on; while it is off the feed is skipped.
	Enabled func(ctx context.Context) bool
	// PerRun is how many sector/region pairs are asked per run (default 2).
	PerRun int
}

func (s *SectorFeed) Name() string { return "sectors" }

// Ready is true while AI is switched on.
func (s *SectorFeed) Ready(ctx context.Context) bool { return s.Enabled == nil || s.Enabled(ctx) }

const sectorPrompt = `You list real companies for a job search. Reply with JSON: {"companies": ["..."]}. Rules: at most 40 names; only companies you are sure exist; the name people use, without "Inc" or "Ltd"; companies that employ software engineers; no invented names; an empty list when unsure.`

var sectorSchema = json.RawMessage(`{"type":"object","properties":{"companies":{"type":"array","items":{"type":"string"},"maxItems":40}},"required":["companies"],"additionalProperties":false}`)

func (s *SectorFeed) Next(ctx context.Context, state string) ([]string, string, bool, error) {
	total := len(Sectors) * len(SectorRegions)
	at, _ := strconv.Atoi(state)
	per := s.PerRun
	if per <= 0 {
		per = 2
	}
	var names []string
	seen := map[string]bool{}
	end := min(at+per, total)
	for i := at; i < end; i++ {
		sector, region := Sectors[i/len(SectorRegions)], SectorRegions[i%len(SectorRegions)]
		resp, err := s.LLM.Complete(ctx, llm.Request{
			System: sectorPrompt, User: fmt.Sprintf("Industry: %s\nRegion: %s", sector, region),
			Schema: sectorSchema, MaxTokens: 700,
		})
		if err != nil {
			return names, strconv.Itoa(i), true, err
		}
		var out struct {
			Companies []string `json:"companies"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &out) != nil {
			continue // an unusable answer skips this pair
		}
		for _, n := range out.Companies {
			n = strings.Join(strings.Fields(n), " ")
			if k := strings.ToLower(n); n != "" && !seen[k] {
				seen[k] = true
				names = append(names, n)
			}
		}
	}
	return names, strconv.Itoa(end), end < total, nil
}
