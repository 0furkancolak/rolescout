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

// ExplanationTask is a strong, enriched match that still lacks an explanation.
type ExplanationTask struct {
	WorkspaceID         string
	ProfileID           string
	JobID               string
	DescriptionHash     string
	Title               string
	CompanyName         string
	Score               int
	Reasons             []MatchReason
	Technologies        []string
	MatchedTechnologies []string
	Seniority           string
	Arrangement         string
	Country             string
	// Locale is the workspace owner's language ("tr" or "en").
	Locale string
}

// JobAI is what the local model added for a job, marked as such: it only lists
// the fields the model filled because the rules did not know them.
type JobAI struct {
	JobID          string
	Model          string
	Summary        string
	RemotePolicy   string
	SalaryMin      *int
	SalaryMax      *int
	SalaryCurrency string
	SalaryPeriod   string
	// Fields names the effective values that came from the model: "seniority",
	// "work_arrangement" or "country".
	Fields []string
	// ExtraTechnologies are technologies only the model found.
	ExtraTechnologies []string
}

// Empty reports whether there is nothing worth showing.
func (a JobAI) Empty() bool {
	return a.Summary == "" && a.RemotePolicy == "" && a.SalaryMin == nil && a.SalaryMax == nil &&
		len(a.Fields) == 0 && len(a.ExtraTechnologies) == 0
}
