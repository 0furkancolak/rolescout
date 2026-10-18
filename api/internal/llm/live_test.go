package llm

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestLiveModel runs the golden descriptions through a real local model. It is
// skipped unless LLM_LIVE_URL is set, for example
//
//	LLM_LIVE_URL=http://127.0.0.1:11434/v1 LLM_LIVE_MODEL=qwen3:4b-instruct-2507-q4_K_M go test ./internal/llm -run Live -v
//
// It reports differences from the expectations without failing on them: the
// point is to look, and the labelled evaluation set (admin llm eval) measures.
func TestLiveModel(t *testing.T) {
	base := os.Getenv("LLM_LIVE_URL")
	if base == "" {
		t.Skip("LLM_LIVE_URL is not set")
	}
	model := os.Getenv("LLM_LIVE_MODEL")
	if model == "" {
		model = "qwen3:4b-instruct-2507-q4_K_M"
	}
	c := NewClient(Options{BaseURL: base, Model: model, Timeout: 5 * time.Minute})
	for _, fx := range loadFixtures(t) {
		start := time.Now()
		got, stats, err := Extract(context.Background(), c, Input{Title: fx.Title, Company: fx.Company, Description: fx.Description, MaxChars: 6000, MaxTokens: 400, ResolveCountry: resolve})
		if err != nil {
			t.Errorf("%s: %v", fx.Name, err)
			continue
		}
		t.Logf("%s: %.1fs retries=%d tokens=%d/%d dropped=%+v", fx.Name, time.Since(start).Seconds(), stats.Retries, stats.PromptTokens, stats.CompletionTokens, stats.Dropped)
		if !reflect.DeepEqual(got, fx.Expected) {
			t.Logf("  got:  %+v\n  want: %+v", got, fx.Expected)
		}
	}
}
