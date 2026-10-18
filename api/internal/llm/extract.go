package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrInvalidOutput means the model twice failed to produce a JSON document that
// parses into an Enrichment.
var ErrInvalidOutput = errors.New("llm: model output is not valid")

// Input is one job to read.
type Input struct {
	Model       string
	Title       string
	Company     string
	Description string
	MaxChars    int
	MaxTokens   int
	// ResolveCountry maps a free-text place to an ISO country code (rules win
	// over the model); may be nil.
	ResolveCountry func(place string) string
}

// Stats describes one extraction.
type Stats struct {
	PromptTokens     int
	CompletionTokens int
	Latency          time.Duration
	Retries          int
	Dropped          Dropped
}

// Extract asks the model once (twice if the first reply is not valid JSON),
// then validates and grounds the answer. Transport errors are returned as they
// are: retrying a down server only makes it worse.
func Extract(ctx context.Context, c Completer, in Input) (Enrichment, Stats, error) {
	system, user := BuildPrompt(in.Title, in.Company, in.Description, in.MaxChars)
	var stats Stats
	var parsed Enrichment
	for attempt := 0; attempt < 2; attempt++ {
		sys := system
		if attempt == 1 {
			sys += strictSuffix
		}
		resp, err := c.Complete(ctx, Request{Model: in.Model, System: sys, User: user, Schema: JobSchema(), MaxTokens: in.MaxTokens})
		if err != nil {
			return Enrichment{}, stats, err
		}
		stats.PromptTokens += resp.PromptTokens
		stats.CompletionTokens += resp.CompletionTokens
		stats.Latency += resp.Latency
		if err := decode(resp.Content, &parsed); err == nil {
			stats.Retries = attempt
			clean, dropped := Validate(parsed, in.Title, in.Description, in.ResolveCountry)
			stats.Dropped = dropped
			return clean, stats, nil
		}
		stats.Retries = attempt + 1
	}
	return Enrichment{}, stats, ErrInvalidOutput
}

func decode(content string, e *Enrichment) error {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		return errors.New("not a JSON object")
	}
	*e = Enrichment{}
	return json.Unmarshal([]byte(content), e)
}
