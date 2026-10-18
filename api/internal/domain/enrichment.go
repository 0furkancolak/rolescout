package domain

import (
	"encoding/json"
	"time"
)

// EnrichmentTask is a job waiting for the local model.
type EnrichmentTask struct {
	JobID           string
	Title           string
	CompanyName     string
	Description     string
	DescriptionHash string
	// Location is the job's stored place, passed to the model as context only.
	Location string
}

// EnrichmentResult is what the enricher stores for one task.
type EnrichmentResult struct {
	JobID            string
	DescriptionHash  string
	Model            string
	PromptVersion    int
	SchemaVersion    int
	OK               bool
	Data             json.RawMessage // validated enrichment JSON when OK
	Error            string          // short reason when not OK; never the text
	PromptTokens     int
	CompletionTokens int
	Latency          time.Duration
	// Now is the clock used for the failure backoff.
	Now time.Time
}

// Enrichment is a stored, validated model result for a job.
type Enrichment struct {
	JobID     string
	Model     string
	Data      json.RawMessage
	CreatedAt time.Time
}

// EnrichmentStats summarises the queue for the status screen.
type EnrichmentStats struct {
	Queue        int
	Enriched     int
	Failed       int
	AvgLatencyMs int
	LastError    string
	LastErrorAt  *time.Time
}

// MatchExplanation is a short model-written reason for one match.
type MatchExplanation struct {
	JobID string
	Text  string
}
